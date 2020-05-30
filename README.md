# casso

[![MIT License](https://img.shields.io/apm/l/atomic-design-ui.svg?)](LICENSE)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/lithdew/casso)
[![Discord Chat](https://img.shields.io/discord/697002823123992617)](https://discord.gg/HZEbkeQ)

**casso** is a low-level Go implementation of the popular [Cassowary](https://constraints.cs.washington.edu/cassowary/cassowary-tr.pdf) constraint solving algorithm.
 
**casso** allows you to efficiently and incrementally describe partially-conflicting required/preferential constraints over a set of variables, and solve for a solution against them that is legitimately locally-error-better much like the [simplex algorithm](https://en.wikipedia.org/wiki/Simplex_algorithm).

It is popularly used in Apple's [Auto Layout Visual Format Language](https://developer.apple.com/library/archive/documentation/UserExperience/Conceptual/AutolayoutPG/VisualFormatLanguage.html), and in [Grid Style Sheets](https://gss.github.io/guides/ccss).

## Description

> Linear equality and inequality constraints arise naturally in specifying many aspects of user interfaces, such as requiring that one window be to the left of another, requiring that a pane occupy the leftmost 1/3 of a window, or preferring that an object be contained within a rectangle if possible. Current constraint solvers designed for UI applications cannot efficiently handle simultaneous linear equations and inequalities. This is a major limitation. We describe Cassowary—an incremental algorithm based on the dual simplex method that can solve such systems of constraints efficiently.

Paper written by Greg J. Badros, and Alan Borning. For more information, please check out the paper [here](https://constraints.cs.washington.edu/cassowary/cassowary-tr.pdf).

## Example

```go
s := casso.NewSolver()

containerWidth := casso.New()

childX := casso.New()
childCompWidth := casso.New()

child2X := casso.New()
child2CompWidth := casso.New()

// c1: childX == (50.0 / 1024) * containerWidth
// c2: childCompWidth == (200.0 / 1024) * containerWidth
// c3: childCompWidth >= 200.0
// c4: child2X - childX - childCompWidth == 50
// c5: child2CompWidth == 50 + containerWidth + child2X

c1 := casso.NewConstraint(casso.EQ, 0, childX.T(1.0), containerWidth.T(-50.0/1024))
c2 := casso.NewConstraint(casso.EQ, 0, childCompWidth.T(1.0), containerWidth.T(-200.0/1024))
c3 := casso.NewConstraint(casso.GTE, -200, childCompWidth.T(1.0))
c4 := casso.NewConstraint(casso.EQ, -50, child2X.T(1.0), childX.T(-1.0), childCompWidth.T(-1.0))
c5 := casso.NewConstraint(casso.EQ, 50, child2CompWidth.T(1.0), containerWidth.T(-1.0), child2X.T(1.0))

// Mark 'containerWidth' as an editable variable with strong precedence.
// Suggest 'containerWidth' to take on the value 2048.

require.NoError(t, s.Edit(containerWidth, casso.Strong))
require.NoError(t, s.Suggest(containerWidth, 2048))

// Add constraints to the solver.

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

// Grab computed values.

require.EqualValues(t, 2048, s.Val(containerWidth))
require.EqualValues(t, 400, s.Val(childCompWidth))
require.EqualValues(t, 1448, s.Val(child2CompWidth))

// Suggest 'containerWidth' to take on the value 500.

require.NoError(t, s.Suggest(containerWidth, 500))

// Grab computed values.

require.EqualValues(t, 500, s.Val(containerWidth))
require.EqualValues(t, 200, s.Val(childCompWidth))
require.EqualValues(t, 175.5859375, s.Val(child2CompWidth))
```

## Remarks

Symbols/references to variables are represented as unsigned 32-bit integers. The first two bits of a symbol denote the symbols type, with the rest of the bits denoting the symbols ID.

A symbol with an ID of zero is marked to be invalid. As a result, a program at any given moment in time may only generate at most 2^30 - 1 symbols, or 1,073,741,824 symbols.

This was done for performance reasons to minimize memory usage and reduce the number of cycles needed to perform some operations. If you need this restriction lifted for a particular reason or use case, please open up a Github issue.

## Benchmarks

```
$ cat /proc/cpuinfo | grep 'model name' | uniq
model name : Intel(R) Core(TM) i7-7700HQ CPU @ 2.80GHz

$ go test -bench=. -benchtime=10s
goos: linux
goarch: amd64
pkg: github.com/lithdew/casso
BenchmarkAddConstraint-8         7102137              2038 ns/op            1024 B/op         11 allocs/op
```