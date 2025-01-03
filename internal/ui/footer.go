package ui

import (
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Footer struct {
	TextView *tview.TextView
}

func NewFooter() *Footer {
	return &Footer{
		TextView: tview.NewTextView().SetDynamicColors(true),
	}
}

func CreateFooterHome() *Footer {
	f := NewFooter()
	f.TextView.SetBackgroundColor(tcell.GetColor(userTheme.Footer.Background))
	f.TextView.SetText(
		createSection("?", "help") +
			createSection("ESC", "quit") +
			createSection("1", "running") +
			createSection("2", "all") +
			createSection("C-d", "remove") +
			createSection("C-r", "start") +
			createSection("C-s", "stop"),
	)
	return f
}

func CreateFooterLogs() *Footer {
	f := NewFooter()
	f.TextView.SetBackgroundColor(tcell.GetColor(userTheme.Footer.Background))
	f.TextView.SetText(logsFooterText())
	return f
}

func logsFooterText() string {
	return createSection("?", "help") +
		createSection("e", "environment") +
		createSection("v", "shell") +
		createSection("Scroll", strconv.FormatBool(ScrollOnNewLogEntry))
}

func (footer *Footer) updateLogsFooter() {
	footer.TextView.SetText(logsFooterText())
}

func createSection(hint string, text string) string {
	section := ("[" +
		userTheme.Footer.Hint +
		":" +
		userTheme.Footer.Background +
		":b] " +
		hint +
		" [" +
		userTheme.Footer.Text +
		":#24292f:B]" +
		text + " ")
	return section
}
