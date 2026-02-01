package screens

import (
	"github.com/charmbracelet/bubbles/viewport"
)

// RenderHomebase renders the homebase screen content
// This screen shows iteration logs and Ralph activity
func RenderHomebase(vp viewport.Model) string {
	return vp.View()
}
