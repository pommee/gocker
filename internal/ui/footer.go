package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func CreateFooterHome() *tview.TextView {

	footer := tview.NewTextView().SetDynamicColors(true)
	footer.SetBackgroundColor(tcell.GetColor(theme.Footer.Background))
	footer.SetText(
		createSection("?", "help") +
			createSection("ESC", "quit") +
			createSection("1", "running") +
			createSection("2", "all"),
	)
	return footer
}

func CreateFooterLogs() *tview.TextView {
	footer := tview.NewTextView().SetDynamicColors(true)
	footer.SetBackgroundColor(tcell.GetColor(theme.Footer.Background))
	footer.SetText(
		createSection("ESC", "back") +
			createSection("ENTER", "search") +
			createSection("A", "attributes") +
			createSection("E", "environment"),
	)
	return footer
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
		":#24292f:B]" +
		text + " ")
	return section
}
