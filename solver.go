package casso

import (
	"errors"
	"fmt"
	"math"
)

type Tag struct {
	priority Priority
	marker   Symbol
	other    Symbol
}

type Edit struct {
	tag Tag
	val float64
}

type Solver struct {
	tabs  map[Symbol]Constraint // symbol id -> constraint
	edits map[Symbol]Edit       // variable id -> value
	tags  map[Symbol]Tag        // marker id -> tag

	infeasible []Symbol

	objective  Expr
	artificial Expr
}

func NewSolver() *Solver {
	return &Solver{
		tabs:  make(map[Symbol]Constraint),
		edits: make(map[Symbol]Edit),
		tags:  make(map[Symbol]Tag),
	}
}

func (s *Solver) Val(id Symbol) float64 {
	row, ok := s.tabs[id]
	if !ok {
		return 0
	}
	return row.expr.constant
}

func (s *Solver) AddConstraint(cell Constraint) (Symbol, error) {
	return s.AddConstraintWithPriority(Required, cell)
}

func (s *Solver) AddConstraintWithPriority(priority Priority, cell Constraint) (Symbol, error) {
	tag := Tag{priority: priority}

	c := cell
	c.expr.terms = make([]Term, 0, len(c.expr.terms))

	// 1. filter away terms with coefficients that are zero
	// 2. check that all variables in the constraint are registered
	// 3. replace variables with their values if they have values assigned to them

	for _, term := range cell.expr.terms {
		if eqz(term.coeff) {
			continue
		}
		if term.id.Zero() {
			return zero, errors.New("symbol referenced in term is nil")
		}
		resolved, exists := s.tabs[term.id]
		if !exists {
			c.expr.addSymbol(term.coeff, term.id)
			continue
		}
		c.expr.addExpr(term.coeff, resolved.expr)
	}

	// convert constraint to augmented simplex form

	switch c.op {
	case LTE, GTE:
		coeff := 1.0
		if c.op == GTE {
			coeff = -1.0
		}

		tag.marker = next(Slack)
		c.expr.addSymbol(coeff, tag.marker)

		if priority < Required {
			tag.other = next(Error)
			c.expr.addSymbol(-coeff, tag.other)
			s.objective.addSymbol(float64(priority), tag.other)
		}
	case EQ:
		if priority < Required {
			tag.marker = next(Error)
			tag.other = next(Error)

			c.expr.addSymbol(-1.0, tag.marker)
			c.expr.addSymbol(1.0, tag.other)

			s.objective.addSymbol(float64(priority), tag.marker)
			s.objective.addSymbol(float64(priority), tag.other)
		} else {
			tag.marker = next(Dummy)
			c.expr.addSymbol(1.0, tag.marker)
		}
	}

	if c.expr.constant < 0.0 {
		c.expr.negate()
	}

	// find a subject variable to pivot on

	subject, err := s.findSubject(c, tag)
	if err != nil {
		return zero, err
	}

	if subject.Zero() {
		err := s.augmentArtificialVariable(c)
		if err != nil {
			return tag.marker, err
		}
	} else {
		// 1. solve for the subject variable
		// 2. substitute the solution into our tableau

		c.expr.solveFor(subject)

		s.substitute(subject, c.expr)
		s.tabs[subject] = c
	}

	s.tags[tag.marker] = tag

	return tag.marker, s.optimizeAgainst(&s.objective)
}

func (s *Solver) RemoveConstraint(marker Symbol) error {
	tag, exists := s.tags[marker]
	if !exists {
		return errors.New("tag is unregistered")
	}

	delete(s.tags, tag.marker)

	if tag.marker.Error() {
		row, exists := s.tabs[tag.marker]
		if exists {
			s.objective.addExpr(float64(-tag.priority), row.expr)
		} else {
			s.objective.addSymbol(float64(-tag.priority), tag.marker)
		}
	}

	if tag.other.Error() {
		row, exists := s.tabs[tag.other]
		if exists {
			s.objective.addExpr(float64(-tag.priority), row.expr)
		} else {
			s.objective.addSymbol(float64(-tag.priority), tag.other)
		}
	}

	row, exists := s.tabs[tag.marker]
	if !exists {
		r1 := math.MaxFloat64
		r2 := math.MaxFloat64

		exit := zero
		first := zero
		second := zero
		third := zero

		for symbol, row := range s.tabs {
			idx := row.expr.find(tag.marker)
			if idx == -1 {
				continue
			}

			coeff := row.expr.terms[idx].coeff
			if eqz(coeff) {
				continue
			}

			if symbol.External() {
				third = symbol
			} else {
				r := -row.expr.constant / coeff

				switch {
				case coeff < 0 && r < r1:
					r1, first = r, symbol
				case coeff >= 0 && r < r2:
					r2, second = r, symbol
				}
			}
		}

		switch {
		case !first.Zero():
			exit = first
		case !second.Zero():
			exit = second
		default:
			exit = third
		}

		row = s.tabs[exit]
		delete(s.tabs, exit)

		row.expr.solveForSymbols(exit, tag.marker)
		s.substitute(tag.marker, row.expr)

		return s.optimizeAgainst(&s.objective)
	}

	delete(s.tabs, tag.marker)

	return s.optimizeAgainst(&s.objective)
}

func (s *Solver) Edit(id Symbol, priority Priority) error {
	if priority < 0 || priority >= Required {
		return errors.New("priority must be non-negative and not required for edit variables")
	}
	if _, exists := s.edits[id]; exists {
		return nil
	}
	constraint := Constraint{op: EQ, expr: NewExpr(0.0, id.T(1.0))}
	marker, err := s.AddConstraintWithPriority(priority, constraint)
	if err != nil {
		return err
	}
	s.edits[id] = Edit{tag: s.tags[marker], val: 0.0}
	return nil
}

func (s *Solver) Suggest(id Symbol, val float64) error {
	edit, ok := s.edits[id]
	if !ok {
		return fmt.Errorf("symbol id %d is not registered as editable", id)
	}

	defer s.optimizeDualObjective()

	delta := val - edit.val

	edit.val = val
	s.edits[id] = edit

	row, exists := s.tabs[edit.tag.marker]
	if exists {
		row.expr.constant -= delta
		if row.expr.constant < 0.0 {
			s.infeasible = append(s.infeasible, edit.tag.marker)
		}
		s.tabs[edit.tag.marker] = row
		return nil
	}

	row, exists = s.tabs[edit.tag.other]
	if exists {
		row.expr.constant -= delta
		if row.expr.constant < 0.0 {
			s.infeasible = append(s.infeasible, edit.tag.other)
		}
		s.tabs[edit.tag.other] = row
		return nil
	}

	for symbol := range s.tabs {
		row := s.tabs[symbol]

		idx := row.expr.find(edit.tag.marker)
		if idx == -1 {
			continue
		}

		coeff := row.expr.terms[idx].coeff
		if eqz(coeff) {
			continue
		}

		row.expr.constant += coeff * delta
		s.tabs[symbol] = row

		if row.expr.constant >= 0.0 {
			continue
		}

		if symbol.External() {
			continue
		}

		s.infeasible = append(s.infeasible, symbol)
	}

	return nil
}

// findSubject finds a subject variable to pivot on. It must either:
// 1. be an external variable,
// 2. be a negative slack/error variable, or
// 3. be a dummy variable that has previously been cancelled out
func (s *Solver) findSubject(cell Constraint, tag Tag) (Symbol, error) {
	for _, term := range cell.expr.terms {
		if term.id.External() {
			return term.id, nil
		}
	}

	if tag.marker.Restricted() {
		idx := cell.expr.find(tag.marker)
		if idx != -1 && cell.expr.terms[idx].coeff < 0.0 {
			return tag.marker, nil
		}
	}

	if tag.other.Restricted() {
		idx := cell.expr.find(tag.other)
		if idx != -1 && cell.expr.terms[idx].coeff < 0.0 {
			return tag.other, nil
		}
	}

	for _, term := range cell.expr.terms {
		if !term.id.Dummy() {
			return zero, nil
		}
	}

	if !eqz(cell.expr.constant) {
		return zero, errors.New("non-zero dummy variable: constraint is unsatisfiable")
	}

	return tag.marker, nil
}

func (s *Solver) substitute(id Symbol, expr Expr) {
	for symbol := range s.tabs {
		row := s.tabs[symbol]
		row.expr.substitute(id, expr)
		s.tabs[symbol] = row
		if symbol.External() || row.expr.constant >= 0.0 {
			continue
		}
		s.infeasible = append(s.infeasible, symbol)
	}
	s.objective.substitute(id, expr)
	s.artificial.substitute(id, expr)
}

func (s *Solver) optimizeAgainst(objective *Expr) error {
	for {
		entry := zero
		exit := zero

		for _, term := range objective.terms {
			if !term.id.Dummy() && term.coeff < 0.0 {
				entry = term.id
				break
			}
		}
		if entry.Zero() {
			return nil
		}

		ratio := math.MaxFloat64

		for symbol := range s.tabs {
			if symbol.External() {
				continue
			}
			idx := s.tabs[symbol].expr.find(entry)
			if idx == -1 {
				continue
			}
			coeff := s.tabs[symbol].expr.terms[idx].coeff
			if coeff >= 0.0 {
				continue
			}
			r := -s.tabs[symbol].expr.constant / coeff
			if r < ratio {
				ratio, exit = r, symbol
			}
		}

		row := s.tabs[exit]
		delete(s.tabs, exit)

		row.expr.solveForSymbols(exit, entry)

		s.substitute(entry, row.expr)
		s.tabs[entry] = row
	}
}

func (s *Solver) augmentArtificialVariable(row Constraint) error {
	art := next(Slack)

	s.tabs[art] = row.clone()
	s.artificial = row.expr.clone()

	err := s.optimizeAgainst(&s.artificial)
	if err != nil {
		return err
	}

	success := eqz(s.artificial.constant)
	s.artificial = NewExpr(0.0)

	artificial, ok := s.tabs[art]
	if ok {
		delete(s.tabs, art)

		if len(artificial.expr.terms) == 0 {
			return nil
		}

		entry := zero
		for _, term := range artificial.expr.terms {
			if term.id.Restricted() {
				entry = term.id
				break
			}
		}
		if entry.Zero() {
			return errors.New("unsatisfiable")
		}

		artificial.expr.solveForSymbols(art, entry)

		s.substitute(entry, artificial.expr)
		s.tabs[entry] = artificial
	}

	for symbol, row := range s.tabs {
		idx := row.expr.find(art)
		if idx == -1 {
			continue
		}
		row.expr.delete(idx)
		s.tabs[symbol] = row
	}

	idx := s.objective.find(art)
	if idx != -1 {
		s.objective.delete(idx)
	}

	if !success {
		return errors.New("unsatisfiable")
	}
	return nil
}

// optimizeDualObjective optimizes away infeasible constraints.
func (s *Solver) optimizeDualObjective() {
	for len(s.infeasible) > 0 {
		exit := s.infeasible[len(s.infeasible)-1]
		s.infeasible = s.infeasible[:len(s.infeasible)-1]

		row, exists := s.tabs[exit]
		if !exists || row.expr.constant >= 0.0 {
			continue
		}

		delete(s.tabs, exit)

		entry := zero
		ratio := math.MaxFloat64

		for _, term := range row.expr.terms {
			if term.coeff <= 0.0 || term.id.Dummy() {
				continue
			}
			idx := s.objective.find(term.id)
			if idx == -1 {
				continue
			}
			r := s.objective.terms[idx].coeff / term.coeff
			if r < ratio {
				entry, ratio = term.id, r
			}
		}

		row.expr.solveForSymbols(exit, entry)

		s.substitute(entry, row.expr)
		s.tabs[entry] = row
	}
}
