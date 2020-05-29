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

	_, err := s.AddConstraint(Constraint{op: EQ, expr: NewExpr(0, r.T(1.0), l.T(1.0), m.T(-2.0))})
	require.NoError(t, err)

	rx := s.rows[r]

	// Constraint (-100 + x_r - x_l >= 0)

	res := Constraint{op: GTE}
	res.expr.constant = -100
	res.expr.addExpr(1.0, rx.cell.expr)
	res.expr.addSymbol(-1.0, l)

	require.Len(t, res.expr.terms, 3) // 3 terms

	require.EqualValues(t, l.T(-2), res.expr.terms[0])                // -2 * x_l
	require.EqualValues(t, m.T(2), res.expr.terms[1])                 // 2 * x_m
	require.EqualValues(t, Term{coeff: -1, id: 3}, res.expr.terms[2]) // -1 * d
}

func TestConstraint(t *testing.T) {
	s := NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	a := Constraint{op: EQ, expr: NewExpr(0, r.T(1.0), l.T(1.0), m.T(-2.0))}
	b := Constraint{op: GTE, expr: NewExpr(-100, r.T(1.0), l.T(-1.0))}
	c := Constraint{op: GTE, expr: NewExpr(0, l.T(1.0))}

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

	a := Constraint{op: EQ, expr: NewExpr(0, r.T(1.0), l.T(1.0), m.T(-2.0))}
	b := Constraint{op: GTE, expr: NewExpr(-100, r.T(1.0), l.T(-1.0))}
	c := Constraint{op: GTE, expr: NewExpr(0, l.T(1.0))}

	_, err := s.AddConstraint(a)
	require.NoError(t, err)

	_, err = s.AddConstraint(b)
	require.NoError(t, err)

	_, err = s.AddConstraint(c)
	require.NoError(t, err)

	// Suggest that 'l' should have a value of 100.

	require.NoError(t, s.Edit(l, Strong))
	require.NoError(t, s.Suggest(l, 100))

	require.EqualValues(t, 100, s.rows[l].cell.expr.constant)
	require.EqualValues(t, 150, s.rows[m].cell.expr.constant)
	require.EqualValues(t, 200, s.rows[r].cell.expr.constant)
}

func TestConstraintRequiringArtificialVariable(t *testing.T) {
	s := NewSolver()

	p1 := s.New()
	p2 := s.New()
	p3 := s.New()

	container := s.New()

	require.NoError(t, s.Edit(container, Strong))
	require.NoError(t, s.Suggest(container, 100.0))

	c1 := NewConstraint(GTE, -30.0, p1.T(1.0))
	c2 := NewConstraint(EQ, 0, p1.T(1), p3.T(-1.0))
	c3 := NewConstraint(EQ, 0, p2.T(1.0), p1.T(-2.0))
	c4 := NewConstraint(EQ, 0.0, container.T(1.0), p1.T(-1.0), p2.T(-1.0), p3.T(-1.0))

	_, err := s.AddConstraintWithPriority(Strong, c1)
	require.NoError(t, err)

	_, err = s.AddConstraintWithPriority(Medium, c2)
	require.NoError(t, err)

	_, err = s.AddConstraint(c3)
	require.NoError(t, err)

	_, err = s.AddConstraint(c4)
	require.NoError(t, err)

	require.EqualValues(t, 30, s.Val(p1))
	require.EqualValues(t, 60, s.Val(p2))
	require.EqualValues(t, 10, s.Val(p3))
	require.EqualValues(t, 100, s.Val(container))
}

func TestComplexConstraints(t *testing.T) {
	s := NewSolver()

	containerWidth := s.New()

	childX := s.New()
	childCompWidth := s.New()

	child2X := s.New()
	child2CompWidth := s.New()

	c1 := NewConstraint(EQ, 0, childX.T(1.0), containerWidth.T(-50.0/1024))
	c2 := NewConstraint(EQ, 0, childCompWidth.T(1.0), containerWidth.T(-200.0/1024))
	c3 := NewConstraint(GTE, -200, childCompWidth.T(1.0))
	c4 := NewConstraint(EQ, -50, child2X.T(1.0), childX.T(-1.0), childCompWidth.T(-1.0))
	c5 := NewConstraint(EQ, 50, child2CompWidth.T(1.0), containerWidth.T(-1.0), child2X.T(1.0))

	require.NoError(t, s.Edit(containerWidth, Strong))
	require.NoError(t, s.Suggest(containerWidth, 2048))

	_, err := s.AddConstraint(c1)
	require.NoError(t, err)

	_, err = s.AddConstraintWithPriority(Weak, c2)
	require.NoError(t, err)

	_, err = s.AddConstraintWithPriority(Strong, c3)
	require.NoError(t, err)

	_, err = s.AddConstraint(c4)
	require.NoError(t, err)

	_, err = s.AddConstraint(c5)
	require.NoError(t, err)

	require.EqualValues(t, 2048, s.Val(containerWidth))
	require.EqualValues(t, 400, s.Val(childCompWidth))
	require.EqualValues(t, 1448, s.Val(child2CompWidth))

	require.NoError(t, s.Suggest(containerWidth, 500))

	require.EqualValues(t, 500, s.Val(containerWidth))
	require.EqualValues(t, 200, s.Val(childCompWidth))
	require.EqualValues(t, 175.5859375, s.Val(child2CompWidth))
}

func BenchmarkAddConstraint(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s := NewSolver()
		l := s.New()
		m := s.New()
		r := s.New()
		a := Constraint{op: EQ, expr: NewExpr(0, l.T(1), r.T(1), m.T(-2))}
		b := Constraint{op: GTE, expr: NewExpr(-10, r.T(1), l.T(-1))}
		s.AddConstraint(a)
		s.AddConstraint(b)
	}
}
