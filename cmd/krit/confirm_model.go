package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// confirmModel is a reusable sub-model for yes/no confirmation prompts.
// It handles cursor navigation (left/right, y/n) and renders a pair of
// styled buttons.
type confirmModel struct {
	title       string
	description string
	cursor      int // 0=yes, 1=no
}

func newConfirmModel(title, description string, defaultYes bool) confirmModel {
	cursor := 1
	if defaultYes {
		cursor = 0
	}
	return confirmModel{
		title:       title,
		description: description,
		cursor:      cursor,
	}
}

// Update processes a key message and returns the updated model, whether
// the user has answered, and the answer value (true=yes, false=no).
func (c confirmModel) Update(msg tea.KeyMsg) (confirmModel, bool, bool) {
	switch msg.String() {
	case "left", "h", "y", "Y":
		c.cursor = 0
	case "right", "l", "n", "N":
		c.cursor = 1
	case "enter", " ":
		return c, true, c.cursor == 0
	}
	return c, false, false
}

// View renders the confirmation prompt at the given width.
func (c confirmModel) View(width int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(c.title))
	b.WriteString("\n\n")
	b.WriteString(c.description)
	b.WriteString("\n\n")

	yes, no := confirmButtons(c.cursor)
	b.WriteString("  " + yes + "   " + no + "\n\n")
	b.WriteString(dimStyle.Render("←/→ y/n pick · enter confirm · q quit"))
	return b.String()
}

// confirmButtons renders a [ Yes ]  [ No ] pair with the active cursor highlighted.
func confirmButtons(cursor int) (string, string) {
	if cursor == 0 {
		return selectedStyle.Render("[ Yes ]"), dimStyle.Render("  No  ")
	}
	return dimStyle.Render("  Yes  "), selectedStyle.Render("[ No ]")
}
