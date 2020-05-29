package cassowary

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddConstraint(t *testing.T) {
	s := NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	_, err := s.AddConstraint(Constraint{op: EQ, expr: NewExpr(0, r.Term(1.0), l.Term(1.0), m.Term(-2.0))})
	require.NoError(t, err)

	rx := s.rows[r]

	// Constraint (-100 + x_r - x_l >= 0)

	res := Constraint{op: GTE}
	res.expr.constant = -100
	res.expr.addExpr(1.0, rx.cell.expr)
	res.expr.addSymbol(-1.0, l)

	require.Len(t, res.expr.terms, 3) // 3 terms

	require.EqualValues(t, l.Term(-2), res.expr.terms[0])             // -2 * x_l
	require.EqualValues(t, m.Term(2), res.expr.terms[1])              // 2 * x_m
	require.EqualValues(t, Term{coeff: -1, id: 3}, res.expr.terms[2]) // -1 * d
}

func TestConstraint(t *testing.T) {
	s := NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	a := Constraint{op: EQ, expr: NewExpr(0, r.Term(1.0), l.Term(1.0), m.Term(-2.0))}
	b := Constraint{op: GTE, expr: NewExpr(-100, r.Term(1.0), l.Term(-1.0))}
	c := Constraint{op: GTE, expr: NewExpr(0, l.Term(1.0))}

	_, err := s.AddConstraint(a)
	require.NoError(t, err)

	_, err = s.AddConstraint(b)
	require.NoError(t, err)

	_, err = s.AddConstraint(c)
	require.NoError(t, err)

	require.EqualValues(t, 0, s.rows[l].cell.expr.constant)
	require.EqualValues(t, 50, s.rows[m].cell.expr.constant)
	require.EqualValues(t, 100, s.rows[r].cell.expr.constant)
}

func TestEditableConstraint(t *testing.T) {
	s := NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	a := Constraint{op: EQ, expr: NewExpr(0, r.Term(1.0), l.Term(1.0), m.Term(-2.0))}
	b := Constraint{op: GTE, expr: NewExpr(-100, r.Term(1.0), l.Term(-1.0))}
	c := Constraint{op: GTE, expr: NewExpr(0, l.Term(1.0))}

	_, err := s.AddConstraint(a)
	require.NoError(t, err)

	_, err = s.AddConstraint(b)
	require.NoError(t, err)

	_, err = s.AddConstraint(c)
	require.NoError(t, err)

	// Suggest that 'l' should have a value of 100.

	require.NoError(t, s.Edit(l, Strong))
	s.Suggest(l, 100)

	require.EqualValues(t, 100, s.rows[l].cell.expr.constant)
	require.EqualValues(t, 150, s.rows[m].cell.expr.constant)
	require.EqualValues(t, 200, s.rows[r].cell.expr.constant)
}

func TestConstraintRequiringArtificialVariable(t *testing.T) {
	s := NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	a := Constraint{op: EQ, expr: NewExpr(0, l.Term(1), r.Term(1), m.Term(-2))}
	b := Constraint{op: GTE, expr: NewExpr(-10, r.Term(1), l.Term(-1))}
	c := Constraint{op: GTE, expr: NewExpr(100, r.Term(-1))}
	d := Constraint{op: GTE, expr: NewExpr(0, l.Term(1))}

	_, err := s.AddConstraint(a)
	require.NoError(t, err)

	_, err = s.AddConstraint(b)
	require.NoError(t, err)

	_, err = s.AddConstraint(c)
	require.NoError(t, err)

	_, err = s.AddConstraint(d)
	require.NoError(t, err)
}

func BenchmarkAddConstraint(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s := NewSolver()
		l := s.New()
		m := s.New()
		r := s.New()
		a := Constraint{op: EQ, expr: NewExpr(0, l.Term(1), r.Term(1), m.Term(-2))}
		b := Constraint{op: GTE, expr: NewExpr(-10, r.Term(1), l.Term(-1))}
		s.AddConstraint(a)
		s.AddConstraint(b)
	}
}
