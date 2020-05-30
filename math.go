package casso

import "sync/atomic"

type SymbolKind uint8

const (
	External SymbolKind = iota
	Slack
	Error
	Dummy
)

var SymbolTable = [...]string{
	External: "External",
	Slack:    "Slack",
	Error:    "Error",
	Dummy:    "Dummy",
}

func (s SymbolKind) Restricted() bool { return s == Slack || s == Error }
func (s SymbolKind) String() string   { return SymbolTable[s] }

type Symbol uint64

var (
	count uint64
	zero  Symbol
)

func New() Symbol {
	return next(External)
}

func next(typ SymbolKind) Symbol {
	return Symbol((atomic.AddUint64(&count, 1) & 0x3fffffffffffffff) | (uint64(typ) << 62))
}

func (sym Symbol) Kind() SymbolKind { return SymbolKind(sym >> 62) }
func (sym Symbol) Zero() bool       { return sym == zero }
func (sym Symbol) Restricted() bool { return !sym.Zero() && sym.Kind().Restricted() }
func (sym Symbol) External() bool   { return !sym.Zero() && sym.Kind() == External }
func (sym Symbol) Slack() bool      { return !sym.Zero() && sym.Kind() == Slack }
func (sym Symbol) Error() bool      { return !sym.Zero() && sym.Kind() == Error }
func (sym Symbol) Dummy() bool      { return !sym.Zero() && sym.Kind() == Dummy }

func (sym Symbol) T(coeff float64) Term { return Term{coeff: coeff, id: sym} }

func (sym Symbol) EQ(val float64) Constraint  { return NewConstraint(EQ, -val, sym.T(1.0)) }
func (sym Symbol) GTE(val float64) Constraint { return NewConstraint(GTE, -val, sym.T(1.0)) }
func (sym Symbol) LTE(val float64) Constraint { return NewConstraint(LTE, -val, sym.T(1.0)) }

type Priority float64

const (
	Weak     Priority = 1
	Medium            = 1e3 * Weak
	Strong            = 1e3 * Medium
	Required          = 1e3 * Strong
)

type Op uint8

const (
	EQ Op = iota
	GTE
	LTE
)

var OpTable = [...]string{
	EQ:  "=",
	GTE: ">=",
	LTE: "<=",
}

func (o Op) String() string { return OpTable[o] }

type Constraint struct {
	op   Op
	expr Expr
}

func NewConstraint(op Op, constant float64, terms ...Term) Constraint {
	return Constraint{op: op, expr: NewExpr(constant, terms...)}
}

func (c Constraint) clone() Constraint {
	res := Constraint{op: c.op, expr: c.expr.clone()}
	return res
}

type Term struct {
	coeff float64
	id    Symbol
}

type Expr struct {
	constant float64
	terms    []Term
}

func NewExpr(constant float64, terms ...Term) Expr {
	return Expr{constant: constant, terms: terms}
}

func (c Expr) clone() Expr {
	res := Expr{constant: c.constant, terms: make([]Term, len(c.terms))}
	copy(res.terms, c.terms)
	return res
}

func (c Expr) find(id Symbol) int {
	for i := 0; i < len(c.terms); i++ {
		if c.terms[i].id == id {
			return i
		}
	}
	return -1
}

func (c *Expr) delete(idx int) {
	copy(c.terms[idx:], c.terms[idx+1:])
	c.terms = c.terms[:len(c.terms)-1]
}

func (c *Expr) addSymbol(coeff float64, id Symbol) {
	idx := c.find(id)
	if idx == -1 {
		if !eqz(coeff) {
			c.terms = append(c.terms, Term{coeff: coeff, id: id})
		}
		return
	}
	c.terms[idx].coeff += coeff
	if eqz(c.terms[idx].coeff) {
		c.delete(idx)
	}
}

func (c *Expr) addExpr(coeff float64, other Expr) {
	c.constant += coeff * other.constant
	for i := 0; i < len(other.terms); i++ {
		c.addSymbol(coeff*other.terms[i].coeff, other.terms[i].id)
	}
}

func (c *Expr) negate() {
	c.constant = -c.constant
	for i := 0; i < len(c.terms); i++ {
		c.terms[i].coeff = -c.terms[i].coeff
	}
}

func (c *Expr) solveFor(id Symbol) {
	idx := c.find(id)
	if idx == -1 {
		return
	}

	// 1. delete variable symbol entry from expression
	// 2. reverse all signs and divide all coefficients by symbol coefficient

	coeff := -1.0 / c.terms[idx].coeff
	c.delete(idx)

	if coeff == 1.0 {
		return
	}

	c.constant *= coeff
	for i := 0; i < len(c.terms); i++ {
		c.terms[i].coeff *= coeff
	}
}

func (c *Expr) solveForSymbols(lhs, rhs Symbol) {
	c.addSymbol(-1.0, lhs)
	c.solveFor(rhs)
}

func (c *Expr) substitute(id Symbol, other Expr) {
	idx := c.find(id)
	if idx == -1 {
		return
	}
	coeff := c.terms[idx].coeff
	c.delete(idx)
	c.addExpr(coeff, other)
}

func eqz(val float64) bool {
	if val < 0 {
		return -val < 1.0e-8
	}
	return val < 1.0e-8
}
