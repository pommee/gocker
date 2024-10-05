package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type LogSearcher struct {
	textView   *tview.TextView
	inputField *tview.InputField
	matches    []string
	index      int
	mu         sync.Mutex
	searchChan chan string
}

func NewLogSearcher(textView *tview.TextView) *LogSearcher {
	ls := &LogSearcher{
		textView:   textView,
		searchChan: make(chan string, 1),
	}
	go ls.searchWorker()
	return ls
}

func (ls *LogSearcher) searchWorker() {
	for keyword := range ls.searchChan {
		ls.search(keyword)
	}
}

func (ls *LogSearcher) search(keyword string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	text := ls.textView.GetText(true)
	if keyword == "" {
		app.QueueUpdateDraw(func() {
			ls.textView.SetText(text)
			ls.textView.Highlight("")
			ls.textView.SetTitle(" Logs ")
		})
		return
	}

	ls.matches = []string{}
	var highlighted strings.Builder

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if idx := strings.Index(strings.ToLower(line), strings.ToLower(keyword)); idx != -1 {
			regionID := fmt.Sprintf("match%d", i)
			beforeMatch := line[:idx]
			matchedKeyword := line[idx : idx+len(keyword)]
			afterMatch := line[idx+len(keyword):]
			highlighted.WriteString(fmt.Sprintf(`["%s"]%s[orange:black]%s[-:-]%s[""]`,
				regionID,
				beforeMatch,
				matchedKeyword,
				afterMatch,
			))
			ls.matches = append(ls.matches, regionID)
		} else {
			highlighted.WriteString(fmt.Sprintf("[gray]%s[-]", line))
		}
		highlighted.WriteString("\n")
	}

	ls.index = len(ls.matches) - 1
	app.QueueUpdateDraw(func() {
		ls.textView.SetText(highlighted.String())
		if len(ls.matches) > 0 {
			ls.highlightMatch()
		} else {
			ls.textView.SetTitle(" No matches found ")
		}
	})

}

func (ls *LogSearcher) CreateInputField(table *tview.Table, containerID string) *tview.InputField {
	ls.inputField = tview.NewInputField().
		SetFieldTextColor(tcell.ColorWhite).
		SetPlaceholderTextColor(tcell.ColorLightGray).
		SetPlaceholder("Logs...")

	ls.inputField.SetChangedFunc(func(text string) {
		select {
		case ls.searchChan <- text:
		default:
		}
	})

	ls.inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			ls.navigateResults(-1)
		case tcell.KeyEscape:
			DrawLogs(table, containerID)
		}
	})

	return ls.inputField
}

func (ls *LogSearcher) navigateResults(direction int) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if len(ls.matches) == 0 {
		return
	}

	ls.index = (ls.index + direction + len(ls.matches)) % len(ls.matches)
	ls.highlightMatch()
}

func (ls *LogSearcher) highlightMatch() {
	regionID := ls.matches[ls.index]
	ls.textView.Highlight(regionID).ScrollToHighlight()
}

func (ls *LogSearcher) Cleanup() {
	close(ls.searchChan)
}
