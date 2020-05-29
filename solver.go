package casso

import (
	"errors"
	"fmt"
	"math"
)

type Tag struct {
	prio   Priority
	marker SymbolID
	other  SymbolID
}

type Edit struct {
	tag Tag
	val float64
}

type Solver struct {
	counter SymbolID

	symbols map[SymbolID]Symbol     // symbol id -> symbol type
	edits   map[SymbolID]Edit       // variable id -> value
	tabs    map[SymbolID]Constraint // symbol id -> constraint
	tags    map[SymbolID]Tag        // marker id -> tag

	infeasible []SymbolID

	objective  Expr
	artificial Expr
}

func NewSolver() *Solver {
	return &Solver{
		symbols: make(map[SymbolID]Symbol),
		edits:   make(map[SymbolID]Edit),
		tabs:    make(map[SymbolID]Constraint),
		tags:    make(map[SymbolID]Tag),
	}
}

func (s *Solver) New() SymbolID {
	return s.new(External)
}

func (s *Solver) Val(id SymbolID) float64 {
	row, ok := s.tabs[id]
	if !ok {
		return 0
	}
	return row.expr.constant
}

func (s *Solver) new(symbol Symbol) SymbolID {
	id := s.counter
	s.symbols[id] = symbol
	s.counter++
	return id
}

func (s *Solver) substitute(id SymbolID, expr Expr) {
	for symbol := range s.tabs {
		row := s.tabs[symbol]
		row.expr.substitute(id, expr)
		s.tabs[symbol] = row

		if s.symbols[symbol] == External || row.expr.constant >= 0.0 {
			continue
		}

		s.infeasible = append(s.infeasible, symbol)
	}
	s.objective.substitute(id, expr)
	s.artificial.substitute(id, expr)
}

func (s *Solver) AddConstraint(cell Constraint) (SymbolID, error) {
	return s.AddConstraintWithPriority(Required, cell)
}

func (s *Solver) AddConstraintWithPriority(prio Priority, cell Constraint) (SymbolID, error) {
	tag := Tag{prio: prio, marker: InvalidSymbolID, other: InvalidSymbolID}

	c := cell
	c.expr.terms = make([]Term, 0, len(c.expr.terms))

	// 1. filter away terms with coefficients that are zero
	// 2. check that all variables in the constraint are registered
	// 3. replace variables with their values if they have values assigned to them

	for _, term := range cell.expr.terms {
		if zero(term.coeff) {
			continue
		}

		if _, exists := s.symbols[term.id]; !exists {
			return InvalidSymbolID, fmt.Errorf("referenced unknown symbol id %d", term.id)
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
		tag.marker = s.new(Slack)
		c.expr.addSymbol(coeff, tag.marker)

		if prio < Required {
			tag.other = s.new(Error)
			c.expr.addSymbol(-coeff, tag.other)
			s.objective.addSymbol(prio.Val(), tag.other)
		}
	case EQ:
		if prio < Required {
			tag.marker = s.new(Error)
			tag.other = s.new(Error)

			c.expr.addSymbol(-1.0, tag.marker)
			c.expr.addSymbol(1.0, tag.other)

			s.objective.addSymbol(prio.Val(), tag.marker)
			s.objective.addSymbol(prio.Val(), tag.other)
		} else {
			tag.marker = s.new(Dummy)
			c.expr.addSymbol(1.0, tag.marker)
		}
	}

	if c.expr.constant < 0.0 {
		c.expr.negate()
	}

	// find a subject variable to pivot on

	subject, err := s.findSubject(c, tag)
	if err != nil {
		return InvalidSymbolID, err
	}

	if subject == -1 {
		err := s.optimizeAgainstRow(c)
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

func (s *Solver) RemoveConstraint(marker SymbolID) error {
	tag, exists := s.tags[marker]
	if !exists {
		return errors.New("tag is unregistered")
	}

	delete(s.tags, tag.marker)

	if s.symbols[tag.marker] == Error {
		row, exists := s.tabs[tag.marker]
		if exists {
			s.objective.addExpr(-tag.prio.Val(), row.expr)
		} else {
			s.objective.addSymbol(-tag.prio.Val(), tag.marker)
		}
	}

	if s.symbols[tag.other] == Error {
		row, exists := s.tabs[tag.other]
		if exists {
			s.objective.addExpr(-tag.prio.Val(), row.expr)
		} else {
			s.objective.addSymbol(-tag.prio.Val(), tag.other)
		}
	}

	row, exists := s.tabs[tag.marker]
	if !exists {
		exit := InvalidSymbolID

		r1 := math.MaxFloat64
		r2 := math.MaxFloat64

		first := InvalidSymbolID
		second := InvalidSymbolID
		third := InvalidSymbolID

		for symbol, row := range s.tabs {
			idx := row.expr.find(tag.marker)
			if idx == -1 {
				continue
			}
			coeff := row.expr.terms[idx].coeff
			if zero(coeff) {
				continue
			}
			if s.symbols[symbol] == External {
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
		case first != InvalidSymbolID:
			exit = first
		case second != InvalidSymbolID:
			exit = second
		default:
			exit = third
		}

		row = s.tabs[exit]
		delete(s.tabs, exit)
		delete(s.symbols, exit)

		row.expr.solveForSymbols(exit, tag.marker)
		s.substitute(tag.marker, row.expr)

		return s.optimizeAgainst(&s.objective)
	}

	delete(s.tabs, tag.marker)
	delete(s.symbols, tag.marker)

	return s.optimizeAgainst(&s.objective)
}

func (s *Solver) Edit(id SymbolID, prio Priority) error {
	if prio == Required {
		return errors.New("editable variables are not allowed to be required")
	}
	constraint := Constraint{op: EQ, expr: NewExpr(0.0, id.T(1.0))}
	marker, err := s.AddConstraintWithPriority(prio, constraint)
	if err != nil {
		return err
	}
	s.edits[id] = Edit{tag: s.tags[marker], val: 0.0}
	return nil
}

func (s *Solver) Suggest(id SymbolID, val float64) error {
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
		if zero(coeff) {
			continue
		}

		row.expr.constant += coeff * delta
		s.tabs[symbol] = row

		if row.expr.constant >= 0.0 {
			continue
		}

		if s.symbols[symbol] == External {
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
func (s *Solver) findSubject(cell Constraint, tag Tag) (SymbolID, error) {
	for _, term := range cell.expr.terms {
		if s.symbols[term.id] == External {
			return term.id, nil
		}
	}

	if marker := s.symbols[tag.marker]; marker.Restricted() {
		idx := cell.expr.find(tag.marker)
		if idx != -1 && cell.expr.terms[idx].coeff < 0.0 {
			return tag.marker, nil
		}
	}

	if other := s.symbols[tag.other]; other.Restricted() {
		idx := cell.expr.find(tag.other)
		if idx != -1 && cell.expr.terms[idx].coeff < 0.0 {
			return tag.other, nil
		}
	}

	for _, term := range cell.expr.terms {
		if s.symbols[term.id] != Dummy {
			return InvalidSymbolID, nil
		}
	}

	if !zero(cell.expr.constant) {
		return InvalidSymbolID, errors.New("non-zero dummy variable: constraint is unsatisfiable")
	}

	return tag.marker, nil
}

func (s *Solver) optimizeAgainst(objective *Expr) error {
	for {
		entry := InvalidSymbolID
		for _, term := range objective.terms {
			if s.symbols[term.id] == Dummy || term.coeff >= 0.0 {
				continue
			}
			entry = term.id
			break
		}

		if entry == InvalidSymbolID {
			return nil
		}

		exit := InvalidSymbolID
		ratio := math.MaxFloat64

		for symbol := range s.tabs {
			if s.symbols[symbol] == External {
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
		delete(s.symbols, exit)

		row.expr.solveForSymbols(exit, entry)

		s.substitute(entry, row.expr)
		s.tabs[entry] = row
	}
}

func (s *Solver) optimizeAgainstRow(row Constraint) error {
	id := s.new(Slack)

	s.tabs[id] = row
	s.artificial = row.expr

	err := s.optimizeAgainst(&s.artificial)
	if err != nil {
		return err
	}

	success := zero(s.artificial.constant)
	s.artificial = NewExpr(0.0)

	artificial, ok := s.tabs[id]
	if ok {
		delete(s.tabs, id)
		delete(s.symbols, id)

		if len(artificial.expr.terms) == 0 {
			return nil
		}

		entry := InvalidSymbolID
		for _, term := range artificial.expr.terms {
			if !s.symbols[term.id].Restricted() {
				continue
			}
			entry = term.id
			break
		}

		if entry == InvalidSymbolID {
			return errors.New("unsatisfiable")
		}

		artificial.expr.solveForSymbols(id, entry)

		s.substitute(entry, artificial.expr)
		s.tabs[entry] = artificial
	}

	for symbol, row := range s.tabs {
		idx := row.expr.find(id)
		if idx == -1 {
			continue
		}
		row.expr.delete(idx)
		s.tabs[symbol] = row
	}

	idx := s.objective.find(id)
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
		delete(s.symbols, exit)

		entry := InvalidSymbolID
		ratio := math.MaxFloat64

		for _, term := range row.expr.terms {
			if term.coeff <= 0.0 || s.symbols[term.id] == Dummy {
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
