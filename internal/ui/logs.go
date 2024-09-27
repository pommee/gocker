package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func DrawLogs(table *tview.Table, containerID string) {
	textView := createTextView(containerID)
	inputField := createLogsInputField(table, textView, containerID)
	footer := CreateFooterLogs()

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, false).
		AddItem(footer, 1, 1, false)

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			flex.Clear()
			flex.AddItem(inputField, 1, 0, false).
				AddItem(textView, 0, 1, false).
				AddItem(footer, 1, 1, false)
			app.SetFocus(inputField)
		}
		if event.Key() == tcell.KeyEscape {
			DrawHome()
			return nil
		}
		return nil
	})

	app.SetRoot(flex, true).SetFocus(textView)
}

func createTextView(containerID string) *tview.TextView {
	textView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetChangedFunc(func() {
		app.Draw()
	})

	go dockerClient.ListenForNewLogs(containerID, app, textView)

	return textView
}

func createLogsInputField(table *tview.Table, textView *tview.TextView, containerID string) *tview.InputField {
	inputField := tview.NewInputField()
	inputField.SetFieldTextColor(tcell.ColorWhite)
	inputField.SetPlaceholderTextColor(tcell.ColorLightGray)
	inputField.SetPlaceholder("Logs...")

	var currentMatchIndex int
	var matchingRegions []string

	inputField.SetChangedFunc(func(text string) {
		keyword := inputField.GetText()

		if keyword != "" {
			matchingRegions = searchLogs(textView, keyword)
			currentMatchIndex = len(matchingRegions) - 1

			if len(matchingRegions) > 0 {
				displayIndex := currentMatchIndex + 1
				regionID := matchingRegions[currentMatchIndex]
				textView.Highlight(regionID).ScrollToHighlight()
				textView.SetTitle(fmt.Sprintf(" Result %d/%d ", displayIndex, len(matchingRegions)))
			} else {
				textView.Highlight("")
				textView.SetTitle(" No matches found ")
			}
		} else {
			textView.Highlight("")
			textView.SetTitle(" Logs ")
		}
	})

	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if len(matchingRegions) > 0 {
				currentMatchIndex = (currentMatchIndex - 1 + len(matchingRegions)) % len(matchingRegions)
				regionID := matchingRegions[currentMatchIndex]
				textView.Highlight(regionID).ScrollToHighlight()

				displayIndex := len(matchingRegions) - currentMatchIndex
				textView.SetTitle(fmt.Sprintf(" Result %d/%d ", displayIndex, len(matchingRegions)))
			}

		case tcell.KeyEscape:
			DrawLogs(table, containerID)
		}
	})

	return inputField
}

func searchLogs(textView *tview.TextView, keyword string) []string {
	text := textView.GetText(true)
	lines := strings.Split(text, "\n")

	var matchingRegions []string
	var highlightedText strings.Builder

	for index, line := range lines {
		if strings.Contains(line, keyword) {
			regionID := fmt.Sprintf("match%d", index)
			highlightedText.WriteString(fmt.Sprintf(`["%s"]%s[""]`, regionID, highlightLine(line, keyword)))
			matchingRegions = append(matchingRegions, regionID)
		} else {
			highlightedText.WriteString(fmt.Sprintf("[gray:black]%s\n", line))
		}
	}

	textView.SetText(highlightedText.String())
	return matchingRegions
}

func highlightLine(line, keyword string) string {
	var highlightedLine strings.Builder
	parts := strings.Split(line, keyword)

	for i, part := range parts {
		if i > 0 {
			highlightedLine.WriteString(fmt.Sprintf("[orange:black]%s[white:black]", keyword))
		}
		highlightedLine.WriteString(fmt.Sprintf("[white:black]%s", part))
	}
	highlightedLine.WriteString("\n")
	return highlightedLine.String()
}
