package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func DrawLogs(table *tview.Table, containerID string) {
	textView := createTextView()
	inputField := createInputField(table, textView, containerID)
	footer := CreateFooterLogs()

	ctx, cancel := context.WithCancel(context.Background())
	go dockerClient.ListenForNewLogs(ctx, containerID, app, textView)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, false).
		AddItem(footer, 1, 1, false)

	var isShellMode bool

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isShellMode {
			return event
		}

		switch event.Key() {
		case tcell.KeyEnter:
			flex.Clear()
			flex.AddItem(inputField, 1, 0, false).
				AddItem(textView, 0, 1, false).
				AddItem(footer, 1, 1, false)
			app.SetFocus(inputField)
		case tcell.KeyEscape:
			cancel()
			DrawHome()
			return nil
		}

		switch event.Rune() {
		case 'a':
			cancel()
			textView.Clear()
			textView.SetText(getAttributes(containerID))
		case 'e':
			cancel()
			textView.Clear()
			textView.SetText(getEnvironmentVariables(containerID))
		case 'v':
			cancel()
			textView.Clear()
			err := dockerClient.CreateContainerShell(context.Background(), containerID, textView)
			if err != nil {
				fmt.Fprintf(textView, "Error creating shell: %v", err)
			} else {
				isShellMode = true
			}
		}

		return event
	})

	app.SetRoot(flex, true).SetFocus(textView)
}

func getAttributes(containerID string) string {
	containerInfo, _ := highlightJSON(dockerClient.GetAttributes(containerID))
	return containerInfo
}

func getEnvironmentVariables(containerID string) string {
	environmentVariables, _ := highlightJSON(dockerClient.GetEnvironmentVariables(containerID))
	return environmentVariables
}

func highlightJSON(jsonStr string) (string, error) {
	var jsonData interface{}

	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		return "", err
	}

	highlighted := new(strings.Builder)
	err = walkAndHighlight(jsonData, highlighted, 0)
	if err != nil {
		return "", err
	}

	return highlighted.String(), nil
}

func walkAndHighlight(data interface{}, builder *strings.Builder, indentLevel int) error {
	indent := strings.Repeat("  ", indentLevel)

	switch v := data.(type) {
	case map[string]interface{}:
		builder.WriteString("{\n")
		for key, value := range v {
			builder.WriteString(fmt.Sprintf("%s[green]\"%s\"[white]: ", indent+"  ", key))
			if err := walkAndHighlight(value, builder, indentLevel+1); err != nil {
				return err
			}
			builder.WriteString(",\n")
		}
		builder.WriteString(indent + "}")
	case []interface{}:
		builder.WriteString("[\n")
		for _, value := range v {
			builder.WriteString(indent + "  ")
			if err := walkAndHighlight(value, builder, indentLevel+1); err != nil {
				return err
			}
			builder.WriteString(",\n")
		}
		builder.WriteString(indent + "]")
	case string:
		builder.WriteString(fmt.Sprintf("[yellow]\"%s\"[white]", v))
	case float64, int:
		builder.WriteString(fmt.Sprintf("[blue]%v[white]", v))
	case bool:
		builder.WriteString(fmt.Sprintf("[magenta]%v[white]", v))
	case nil:
		builder.WriteString("[gray]null[white]")
	default:
		builder.WriteString(fmt.Sprintf("%v", v))
	}

	return nil
}

func createTextView() *tview.TextView {
	textView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetChangedFunc(func() {
		app.Draw()
	})

	return textView
}

func createInputField(table *tview.Table, textView *tview.TextView, containerID string) *tview.InputField {
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
	lowerKeyword := strings.ToLower(keyword)

	for index, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, lowerKeyword) {
			regionID := fmt.Sprintf("match%d", index)
			highlightedLine := highlightAllMatches(line, lowerLine, lowerKeyword)
			highlightedText.WriteString(fmt.Sprintf(`["%s"]%s[""]`, regionID, highlightedLine))
			matchingRegions = append(matchingRegions, regionID)
		} else {
			highlightedText.WriteString(fmt.Sprintf("[gray:black]%s\n", line))
		}
	}

	textView.SetText(highlightedText.String())
	return matchingRegions
}

func highlightAllMatches(line, lowerLine, lowerKeyword string) string {
	var highlightedLine strings.Builder
	start := 0
	keywordLen := len(lowerKeyword)

	for {
		startIndex := strings.Index(lowerLine[start:], lowerKeyword)
		if startIndex == -1 {
			highlightedLine.WriteString(line[start:])
			break
		}

		highlightedLine.WriteString(line[start : start+startIndex])
		highlightedLine.WriteString(fmt.Sprintf("[orange:black]%s[white:black]", line[start+startIndex:start+startIndex+keywordLen]))
		start += startIndex + keywordLen
	}

	highlightedLine.WriteString("\n")
	return highlightedLine.String()
}
