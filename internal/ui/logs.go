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
	var scrollSpeed = 3

	textView := createTextView()
	logSearcher := NewLogSearcher(textView)
	inputField := logSearcher.CreateInputField(table, containerID)
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
			logSearcher.Cleanup()
			cancel()
			DrawHome()
			return nil
		case tcell.KeyUp:
			yOffset, xOffset := textView.GetScrollOffset()
			textView.ScrollTo(yOffset-scrollSpeed, xOffset)
			return nil
		case tcell.KeyDown:
			yOffset, xOffset := textView.GetScrollOffset()
			textView.ScrollTo(yOffset+scrollSpeed, xOffset)
			return nil
		case tcell.KeyPgUp:
			yOffset, xOffset := textView.GetScrollOffset()
			textView.ScrollTo(yOffset-scrollSpeed*3, xOffset)
			return nil
		case tcell.KeyPgDn:
			yOffset, xOffset := textView.GetScrollOffset()
			textView.ScrollTo(yOffset+scrollSpeed*3, xOffset)
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
	textView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		switch action {
		case tview.MouseScrollUp:
			yOffset, xOffset := textView.GetScrollOffset()
			textView.ScrollTo(yOffset-scrollSpeed, xOffset)
			return action, event
		case tview.MouseScrollDown:
			yOffset, xOffset := textView.GetScrollOffset()
			textView.ScrollTo(yOffset+scrollSpeed, xOffset)
			return action, event
		}
		return action, event
	})

	app.SetRoot(flex, true).SetFocus(textView)
}

func createTextView() *tview.TextView {
	textView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetChangedFunc(func() {
		app.Draw()
	})

	return textView
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
