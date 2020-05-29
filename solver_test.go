package casso_test

import (
	"github.com/lithdew/casso"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConstraint(t *testing.T) {
	s := casso.NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	a := casso.NewConstraint(casso.EQ, 0, r.T(1), l.T(1), m.T(-2))
	b := casso.NewConstraint(casso.GTE, -100, r.T(1), l.T(-1))
	c := casso.NewConstraint(casso.GTE, 0, l.T(1))

	_, err := s.AddConstraint(a)
	require.NoError(t, err)

	_, err = s.AddConstraint(b)
	require.NoError(t, err)

	_, err = s.AddConstraint(c)
	require.NoError(t, err)

	require.EqualValues(t, 0, s.Val(l))
	require.EqualValues(t, 50, s.Val(m))
	require.EqualValues(t, 100, s.Val(r))
}

func TestEditableConstraint(t *testing.T) {
	s := casso.NewSolver()
	l := s.New()
	m := s.New()
	r := s.New()

	a := casso.NewConstraint(casso.EQ, 0, r.T(1), l.T(1), m.T(-2))
	b := casso.NewConstraint(casso.GTE, -100, r.T(1), l.T(-1))
	c := casso.NewConstraint(casso.GTE, 0, l.T(1))

	_, err := s.AddConstraint(a)
	require.NoError(t, err)

	_, err = s.AddConstraint(b)
	require.NoError(t, err)

	_, err = s.AddConstraint(c)
	require.NoError(t, err)

	// Suggest that 'l' should have a value of 100.

	require.NoError(t, s.Edit(l, casso.Strong))
	require.NoError(t, s.Suggest(l, 100))

	require.EqualValues(t, 100, s.Val(l))
	require.EqualValues(t, 150, s.Val(m))
	require.EqualValues(t, 200, s.Val(r))
}

func TestConstraintRequiringArtificialVariable(t *testing.T) {
	s := casso.NewSolver()

	p1 := s.New()
	p2 := s.New()
	p3 := s.New()

	container := s.New()

	require.NoError(t, s.Edit(container, casso.Strong))
	require.NoError(t, s.Suggest(container, 100.0))

	c1 := casso.NewConstraint(casso.GTE, -30.0, p1.T(1.0))
	c2 := casso.NewConstraint(casso.EQ, 0, p1.T(1), p3.T(-1.0))
	c3 := casso.NewConstraint(casso.EQ, 0, p2.T(1.0), p1.T(-2.0))
	c4 := casso.NewConstraint(casso.EQ, 0.0, container.T(1.0), p1.T(-1.0), p2.T(-1.0), p3.T(-1.0))

	_, err := s.AddConstraintWithPriority(casso.Strong, c1)
	require.NoError(t, err)

	_, err = s.AddConstraintWithPriority(casso.Medium, c2)
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

func TestPaddingUI(t *testing.T) {
	s := casso.NewSolver()

	sw := s.New() // screen width
	sh := s.New() // screen height

	padding := s.New() // padding

	require.NoError(t, s.Edit(sw, casso.Strong))
	require.NoError(t, s.Edit(sh, casso.Strong))
	require.NoError(t, s.Edit(padding, casso.Strong))

	require.NoError(t, s.Suggest(sw, 800))
	require.NoError(t, s.Suggest(sh, 600))
	require.NoError(t, s.Suggest(padding, 30))

	r := func(c casso.Constraint) {
		_, err := s.AddConstraint(c)
		require.NoError(t, err)
	}

	x := s.New()
	y := s.New()
	w := s.New()
	h := s.New()

	// x >= padding
	// x + width + padding <= screen_width - 1
	// y >= padding
	// y + height + padding <= screen_height - 1

	c1 := casso.NewConstraint(casso.GTE, 0, x.T(1), padding.T(-1))
	c2 := casso.NewConstraint(casso.LTE, 1, x.T(1), w.T(1), padding.T(1), sw.T(-1))
	c3 := casso.NewConstraint(casso.GTE, 0, y.T(1), padding.T(-1))
	c4 := casso.NewConstraint(casso.LTE, 1, y.T(1), h.T(1), padding.T(1), sh.T(-1))

	r(c1)
	r(c2)
	r(c3)
	r(c4)

	require.EqualValues(t, 30, s.Val(x))
	require.EqualValues(t, 30, s.Val(y))
	require.EqualValues(t, 739, s.Val(w))
	require.EqualValues(t, 539, s.Val(h))

	require.NoError(t, s.Suggest(padding, 50))

	require.EqualValues(t, 50, s.Val(x))
	require.EqualValues(t, 50, s.Val(y))
	require.EqualValues(t, 699, s.Val(w))
	require.EqualValues(t, 499, s.Val(h))
}

func TestComplexConstraints(t *testing.T) {
	s := casso.NewSolver()

	containerWidth := s.New()

	childX := s.New()
	childCompWidth := s.New()

	child2X := s.New()
	child2CompWidth := s.New()

	c1 := casso.NewConstraint(casso.EQ, 0, childX.T(1.0), containerWidth.T(-50.0/1024))
	c2 := casso.NewConstraint(casso.EQ, 0, childCompWidth.T(1.0), containerWidth.T(-200.0/1024))
	c3 := casso.NewConstraint(casso.GTE, -200, childCompWidth.T(1.0))
	c4 := casso.NewConstraint(casso.EQ, -50, child2X.T(1.0), childX.T(-1.0), childCompWidth.T(-1.0))
	c5 := casso.NewConstraint(casso.EQ, 50, child2CompWidth.T(1.0), containerWidth.T(-1.0), child2X.T(1.0))

	require.NoError(t, s.Edit(containerWidth, casso.Strong))
	require.NoError(t, s.Suggest(containerWidth, 2048))

	_, err := s.AddConstraint(c1)
	require.NoError(t, err)

	_, err = s.AddConstraintWithPriority(casso.Weak, c2)
	require.NoError(t, err)

	_, err = s.AddConstraintWithPriority(casso.Strong, c3)
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
		s := casso.NewSolver()
		l := s.New()
		m := s.New()
		r := s.New()
		a := casso.NewConstraint(casso.EQ, 0, l.T(1), r.T(1), m.T(-2))
		b := casso.NewConstraint(casso.GTE, -10, r.T(1), l.T(-1))
		s.AddConstraint(a)
		s.AddConstraint(b)
	}
}
