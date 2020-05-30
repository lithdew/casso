package casso

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSymbol(t *testing.T) {
	v := next(External)
	require.False(t, v.Zero())
	require.EqualValues(t, External, v.Kind())

	v = next(Slack)
	require.False(t, v.Zero())
	require.EqualValues(t, Slack, v.Kind())

	v = next(Error)
	require.False(t, v.Zero())
	require.EqualValues(t, Error, v.Kind())

	v = next(Dummy)
	require.False(t, v.Zero())
	require.EqualValues(t, Dummy, v.Kind())
}
