package cassowary

import (
	"errors"
	"fmt"
	"math"
)

type Tag struct {
	marker SymbolID
	other  SymbolID
}

type Row struct {
	priority Priority // remove row, add tagID (int) and tag
	cell     Constraint
}

type Edit struct {
	tag Tag
	val float64
}

type Solver struct {
	counter SymbolID

	symbols map[SymbolID]Symbol // symbol id -> symbol type
	edits   map[SymbolID]Edit   // variable id -> value
	rows    map[SymbolID]Row    // symbol id -> row
	tags    map[SymbolID]Tag    // marker id -> tag

	infeasible []SymbolID

	objective Expr
}

func NewSolver() *Solver {
	return &Solver{
		symbols: make(map[SymbolID]Symbol),
		edits:   make(map[SymbolID]Edit),
		rows:    make(map[SymbolID]Row),
		tags:    make(map[SymbolID]Tag),
	}
}

func (s *Solver) New() SymbolID {
	return s.new(External)
}

func (s *Solver) Val(id SymbolID) float64 {
	row, ok := s.rows[id]
	if !ok {
		return 0
	}
	return row.cell.expr.constant
}

func (s *Solver) Edit(id SymbolID, priority Priority) error {
	if priority == Required {
		return errors.New("editable variables are not allowed to be required")
	}
	constraint := Constraint{op: EQ, expr: NewExpr(0.0, id.Term(1.0))}
	marker, err := s.AddConstraintWithPriority(priority, constraint)
	if err != nil {
		return err
	}
	s.edits[id] = Edit{tag: s.tags[marker], val: 0.0}
	return nil
}

func (s *Solver) Suggest(id SymbolID, val float64) {
	edit, ok := s.edits[id]
	if !ok {
		return
	}

	defer s.optimizeDualObjective()

	delta := val - edit.val
	edit.val = val

	row, exists := s.rows[edit.tag.marker]
	if exists {
		row.cell.expr.constant -= delta
		if row.cell.expr.constant < 0.0 {
			s.infeasible = append(s.infeasible, edit.tag.marker)
		}
		s.rows[edit.tag.marker] = row
		return
	}

	row, exists = s.rows[edit.tag.other]
	if exists {
		row.cell.expr.constant -= delta
		if row.cell.expr.constant < 0.0 {
			s.infeasible = append(s.infeasible, edit.tag.other)
		}
		s.rows[edit.tag.other] = row
		return
	}

	for symbol := range s.rows {
		row := s.rows[symbol]

		idx := row.cell.expr.find(edit.tag.marker)
		if idx == -1 {
			continue
		}

		coeff := row.cell.expr.terms[idx].coeff
		if zero(coeff) {
			continue
		}

		row.cell.expr.constant += coeff * delta
		s.rows[symbol] = row

		if row.cell.expr.constant >= 0.0 {
			continue
		}

		if s.symbols[symbol] == External {
			continue
		}

		s.infeasible = append(s.infeasible, symbol)
	}
}

func (s *Solver) new(symbol Symbol) SymbolID {
	id := s.counter
	s.symbols[id] = symbol
	s.counter++
	return id
}

func (s *Solver) substitute(id SymbolID, expr Expr) {
	for symbol := range s.rows {
		row := s.rows[symbol]
		row.cell.expr.substitute(id, expr)
		s.rows[symbol] = row

		if s.symbols[symbol] == External || row.cell.expr.constant >= 0.0 {
			continue
		}

		s.infeasible = append(s.infeasible, symbol)
	}
	s.objective.substitute(id, expr)
}

func (s *Solver) AddConstraint(cell Constraint) (SymbolID, error) {
	return s.AddConstraintWithPriority(Required, cell)
}

func (s *Solver) AddConstraintWithPriority(priority Priority, cell Constraint) (SymbolID, error) {
	tag := Tag{marker: InvalidSymbolID, other: InvalidSymbolID}

	row := Row{priority: priority, cell: cell}
	row.cell.expr.terms = make([]Term, 0, len(row.cell.expr.terms))

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

		resolved, exists := s.rows[term.id]
		if !exists {
			row.cell.expr.addSymbol(term.coeff, term.id)
			continue
		}
		row.cell.expr.addExpr(term.coeff, resolved.cell.expr)
	}

	// convert constraint to augmented simplex form

	switch row.cell.op {
	case LTE, GTE:
		coeff := 1.0
		if row.cell.op == GTE {
			coeff = -1.0
		}
		tag.marker = s.new(Slack)
		row.cell.expr.addSymbol(coeff, tag.marker)

		if priority < Required {
			tag.other = s.new(Error)
			row.cell.expr.addSymbol(-coeff, tag.other)
			s.objective.addSymbol(priority.Val(), tag.other)
		}
	case EQ:
		if priority < Required {
			tag.marker = s.new(Error)
			tag.other = s.new(Error)

			row.cell.expr.addSymbol(-1.0, tag.marker)
			row.cell.expr.addSymbol(1.0, tag.other)

			s.objective.addSymbol(priority.Val(), tag.marker)
			s.objective.addSymbol(priority.Val(), tag.other)
		} else {
			tag.marker = s.new(Dummy)
			row.cell.expr.addSymbol(1.0, tag.marker)
		}
	}

	if row.cell.expr.constant < 0.0 {
		row.cell.expr.negate()
	}

	// find a subject variable to pivot on

	subject, err := s.findSubject(row, tag)
	if err != nil {
		return InvalidSymbolID, err
	}

	if subject == -1 { // TODO(kenta): add with artificial variable
		panic("TODO")
	}

	// 1. solve for the subject variable
	// 2. substitute the solution into our tableau

	row.cell.expr.solveFor(subject)

	s.substitute(subject, row.cell.expr)

	s.tags[tag.marker] = tag
	s.rows[subject] = row

	return tag.marker, s.optimizeAgainst(s.objective)
}

// findSubject finds a subject variable to pivot on. It must either:
// 1. be an external variable,
// 2. be a negative slack/error variable, or
// 3. be a dummy variable that has previously been cancelled out
func (s *Solver) findSubject(row Row, tag Tag) (SymbolID, error) {
	for _, term := range row.cell.expr.terms {
		if s.symbols[term.id] == External {
			return term.id, nil
		}
	}

	if marker := s.symbols[tag.marker]; marker.Restricted() {
		idx := row.cell.expr.find(tag.marker)
		if idx != -1 && row.cell.expr.terms[idx].coeff < 0.0 {
			return tag.marker, nil
		}
	}

	if other := s.symbols[tag.other]; other.Restricted() {
		idx := row.cell.expr.find(tag.other)
		if idx != -1 && row.cell.expr.terms[idx].coeff < 0.0 {
			return tag.other, nil
		}
	}

	for _, term := range row.cell.expr.terms {
		if s.symbols[term.id] != Dummy {
			return InvalidSymbolID, nil
		}
	}

	if !zero(row.cell.expr.constant) {
		return InvalidSymbolID, errors.New("non-zero dummy variable: constraint is unsatisfiable")
	}

	return tag.marker, nil
}

func (s *Solver) optimizeAgainst(objective Expr) error {
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

		for symbol := range s.rows {
			if s.symbols[symbol] == External {
				continue
			}
			idx := s.rows[symbol].cell.expr.find(entry)
			if idx == -1 {
				continue
			}
			coeff := s.rows[symbol].cell.expr.terms[idx].coeff
			if coeff >= 0.0 {
				continue
			}
			r := -s.rows[symbol].cell.expr.constant / coeff
			if r < ratio {
				ratio, exit = r, symbol
			}
		}

		if exit == InvalidSymbolID {
			panic("this should not happen")
		}

		row := s.rows[exit]
		delete(s.rows, exit)

		row.cell.expr.solveForSymbols(exit, entry)

		s.substitute(entry, row.cell.expr)
		s.rows[entry] = row
	}
}

// optimizeDualObjective optimizes away infeasible constraints.
func (s *Solver) optimizeDualObjective() {
	for len(s.infeasible) > 0 {
		exit := s.infeasible[len(s.infeasible)-1]
		s.infeasible = s.infeasible[:len(s.infeasible)-1]

		row, exists := s.rows[exit]
		if !exists || row.cell.expr.constant >= 0.0 {
			continue
		}

		delete(s.rows, exit)

		entry := InvalidSymbolID
		ratio := math.MaxFloat64

		for _, term := range row.cell.expr.terms {
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

		if entry == InvalidSymbolID {
			panic("this definitely should not happen")
		}

		row.cell.expr.solveForSymbols(exit, entry)

		s.substitute(entry, row.cell.expr)
		s.rows[entry] = row
	}
}
