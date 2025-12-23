package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultFormatterStyles(t *testing.T) {
	s := DefaultFormatterStyles()

	assert.NotNil(t, s.Error)
	assert.NotNil(t, s.Warn)
	assert.NotNil(t, s.Success)
	assert.NotNil(t, s.Header)
	assert.NotNil(t, s.File)
	assert.NotNil(t, s.Muted)
}

func TestStylesSingleton(t *testing.T) {
	s1 := Styles()
	s2 := Styles()
	assert.Same(t, s1, s2)
}

func TestStylesRender(t *testing.T) {
	s := Styles()

	// Verify styles can render without panic
	assert.NotEmpty(t, s.Error.Render("error"))
	assert.NotEmpty(t, s.Warn.Render("warn"))
	assert.NotEmpty(t, s.Success.Render("success"))
	assert.NotEmpty(t, s.Header.Render("header"))
	assert.NotEmpty(t, s.File.Render("file"))
	assert.NotEmpty(t, s.Muted.Render("muted"))
}
