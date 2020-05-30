package casso

type symbolType int

const (
	External symbolType = iota
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

func (s symbolType) Restricted() bool { return s == Slack || s == Error }
func (s symbolType) String() string   { return SymbolTable[s] }

type Symbol struct {
	typ symbolType
}

func New() *Symbol {
	return &Symbol{typ: External}
}

func (id *Symbol) Restricted() bool { return id != nil && id.typ.Restricted() }
func (id *Symbol) External() bool   { return id != nil && id.typ == External }
func (id *Symbol) Slack() bool      { return id != nil && id.typ == Slack }
func (id *Symbol) Error() bool      { return id != nil && id.typ == Error }
func (id *Symbol) Dummy() bool      { return id != nil && id.typ == Dummy }

func (id *Symbol) T(coeff float64) Term { return Term{coeff: coeff, id: id} }

func (id *Symbol) EQ(val float64) Constraint  { return NewConstraint(EQ, -val, id.T(1.0)) }
func (id *Symbol) GTE(val float64) Constraint { return NewConstraint(GTE, -val, id.T(1.0)) }
func (id *Symbol) LTE(val float64) Constraint { return NewConstraint(LTE, -val, id.T(1.0)) }

type Priority float64

const (
	Weak     Priority = 1
	Medium            = 1e3 * Weak
	Strong            = 1e3 * Medium
	Required          = 1e3 * Strong
)

type Op int

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

type Term struct {
	coeff float64
	id    *Symbol
}

type Expr struct {
	constant float64
	terms    []Term
}

func NewExpr(constant float64, terms ...Term) Expr {
	return Expr{constant: constant, terms: terms}
}

func (c Expr) find(id *Symbol) int {
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

func (c *Expr) addSymbol(coeff float64, id *Symbol) {
	idx := c.find(id)
	if idx == -1 {
		if !zero(coeff) {
			c.terms = append(c.terms, Term{coeff: coeff, id: id})
		}
		return
	}
	c.terms[idx].coeff += coeff
	if zero(c.terms[idx].coeff) {
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

func (c *Expr) solveFor(id *Symbol) {
	idx := c.find(id)
	if idx == -1 {
		return
	}

	// 1. delete variable symbol entry from expression
	// 2. reverse all signs and divide all coefficients by symbol coefficient

	coeff := -1.0 / c.terms[idx].coeff
	c.delete(idx)

	c.constant *= coeff
	for i := 0; i < len(c.terms); i++ {
		c.terms[i].coeff *= coeff
	}
}

func (c *Expr) solveForSymbols(lhs, rhs *Symbol) {
	c.addSymbol(-1.0, lhs)
	c.solveFor(rhs)
}

func (c *Expr) substitute(id *Symbol, other Expr) {
	idx := c.find(id)
	if idx == -1 {
		return
	}
	coeff := c.terms[idx].coeff
	c.delete(idx)
	c.addExpr(coeff, other)
}

func zero(val float64) bool {
	if val < 0 {
		return -val < 1.0e-8
	}
	return val < 1.0e-8
}
