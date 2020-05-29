package cassowary

type SymbolID int

const InvalidSymbolID SymbolID = -1

func (id SymbolID) T(coeff float64) Term { return Term{coeff: coeff, id: id} }

func (id SymbolID) EQ(val float64) Constraint  { return NewConstraint(EQ, -val, id.T(1.0)) }
func (id SymbolID) GTE(val float64) Constraint { return NewConstraint(GTE, -val, id.T(1.0)) }
func (id SymbolID) LTE(val float64) Constraint { return NewConstraint(LTE, -val, id.T(1.0)) }

type Priority int

const (
	Weak Priority = iota
	Medium
	Strong
	Required
)

var PriorityTable = [...]float64{
	Weak:     1,
	Medium:   1e3,
	Strong:   1e6,
	Required: 1e9,
}

func (p Priority) Val() float64 { return PriorityTable[p] }

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

type Symbol int

const (
	External Symbol = iota
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

func (s Symbol) Restricted() bool { return s == Slack || s == Error }
func (s Symbol) String() string   { return SymbolTable[s] }

type Constraint struct {
	op   Op
	expr Expr
}

func NewConstraint(op Op, constant float64, terms ...Term) Constraint {
	return Constraint{op: op, expr: NewExpr(constant, terms...)}
}

type Term struct {
	coeff float64
	id    SymbolID
}

type Expr struct {
	constant float64
	terms    []Term
}

func NewExpr(constant float64, terms ...Term) Expr {
	return Expr{constant: constant, terms: terms}
}

func (c Expr) find(id SymbolID) int {
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

func (c *Expr) addSymbol(coeff float64, id SymbolID) {
	idx := c.find(id)
	if idx == -1 {
		c.terms = append(c.terms, Term{coeff: coeff, id: id})
		return
	}
	c.terms[idx].coeff += coeff
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

func (c *Expr) solveForSymbols(lhs, rhs SymbolID) {
	c.addSymbol(-1.0, lhs)
	c.solveFor(rhs)
}

func (c *Expr) solveFor(id SymbolID) {
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

func (c *Expr) substitute(id SymbolID, other Expr) {
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
