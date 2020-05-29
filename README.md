# cassowary

[![MIT License](https://img.shields.io/apm/l/atomic-design-ui.svg?)](LICENSE)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/lithdew/cassowary)
[![Discord Chat](https://img.shields.io/discord/697002823123992617)](https://discord.gg/HZEbkeQ)

**cassowary** is a low-level Go implementation of the popular [Cassowary](https://constraints.cs.washington.edu/cassowary/cassowary-tr.pdf) constraint solving algorithm.

**cassowary** allows you to incrementally describe preferences and requirements of a set of partially-conflicting constraints, and solve for a solution against them that is legitimate locally-error-better much like the [simplex algorithm](https://en.wikipedia.org/wiki/Simplex_algorithm).

**cassowary** is popularly used in Apple's Auto Layout and Visual Format Language, and in Grid Style Sheets.

## Description

> Linear equality and inequality constraints arise naturally in specifying many aspects of user interfaces, such as requiring that one window be to the left of another, requiring that a pane occupy the leftmost 1/3 of a window, or preferring that an object be contained within a rectangle if possible. Current constraint solvers designed for UI applications cannot efficiently handle simultaneous linear equations and inequalities. This is a major limitation. We describe Cassowary—an incremental algorithm based on the dual simplex method that can solve such systems of constraints efficiently.

Paper written by Greg J. Badros, and Alan Borning. For more information, please check out the paper [here](https://constraints.cs.washington.edu/cassowary/cassowary-tr.pdf).

## Example

```go
s := cassowary.NewSolver()
l := s.New()
m := s.New()
r := s.New()

// a: r + l - 2m == 0
// b: r - l >= 100
// c: l >= 0

a := cassowary.Constraint{op: EQ, expr: cassowary.NewExpr(0, r.Term(1.0), l.Term(1.0), m.Term(-2.0))}
b := cassowary.Constraint{op: GTE, expr: cassowary.NewExpr(-100, r.Term(1.0), l.Term(-1.0))}
c := cassowary.Constraint{op: GTE, expr: cassowary.NewExpr(0, l.Term(1.0))}

_, err := s.AddConstraint(a)
require.NoError(t, err)

_, err = s.AddConstraint(b)
require.NoError(t, err)

_, err = s.AddConstraint(c)
require.NoError(t, err)

require.EqualValues(t, 0, s.Val(l))
require.EqualValues(t, 50, s.Val(m))
require.EqualValues(t, 100, s.Val(r))
```

## Benchmarks

```
$ cat /proc/cpuinfo | grep 'model name' | uniq
model name : Intel(R) Core(TM) i7-7700HQ CPU @ 2.80GHz

$ go test -bench=. -benchtime=10s
goos: linux
goarch: amd64
pkg: github.com/lithdew/cassowary
BenchmarkAddConstraint-8         4392736              2753 ns/op            1344 B/op         13 allocs/op
```