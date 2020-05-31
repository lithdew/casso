package casso

import "errors"

var (
	ErrBadPriority         = errors.New("priority must be non-negative and not required for edit variable")
	ErrNotEditVariable     = errors.New("symbol is not yet registered as an edit variable")
	ErrBadTermInConstraint = errors.New("one of the terms in the constraint references a nil symbol")
	ErrBadConstraintMarker = errors.New("symbol is not registered to refer to a constraint")
	ErrBadDummyVariable    = errors.New("constraint is unsatisfiable: non-zero dummy variable")
)
