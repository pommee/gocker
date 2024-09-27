package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func CreateFooter() *tview.TextView {

	header := tview.NewTextView().SetDynamicColors(true)
	header.SetBackgroundColor(tcell.GetColor(theme.Footer.Background))
	header.SetText(
		createSection("?", "help") +
			createSection("ESC", "quit") +
			createSection("1", "running") +
			createSection("2", "all"),
	)
	return header
}

func createSection(hint string, text string) string {
	section := ("[" +
		theme.Footer.Hint +
		":" +
		theme.Footer.Background +
		":b] " +
		hint +
		" [" +
		theme.Footer.Text +
		":#24292f:B] " +
		text + " ")
	return section
}
