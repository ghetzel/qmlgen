package qmlgen

import (
	"testing"

	"github.com/ghetzel/testify/require"
)

func TestGenerateBasic(t *testing.T) {
	assert := require.New(t)

	assert.Equal(`NonexistingThing{}`, NewComponent(`NonexistingThing`).String())

	win := NewComponent(`ApplicationWindow`)
	win.Set(`visible`, true)
	win.Set(`color`, `#FF00CC`)

	assert.Equal("ApplicationWindow {\n  visible: true\n  color: \"#FF00CC\"\n}", win.String())
}
