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
	go dockerClient.ListenForNewLogs(ctx, containerID, app, textView, &ScrollOnNewLogEntry)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, false).
		AddItem(footer.TextView, 1, 1, false)

	var isShellMode bool

	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	helpBox := tview.NewFlex().
		AddItem(helpText(), 0, 1, false)
	helpBox.SetBorder(true)
	helpBox.SetTitle("  Help - Press [orange:-:b]ESC[white:-:B] to exit  ")
	helpBox.SetTitleAlign(tview.AlignCenter)

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isShellMode {
			return event
		}

		switch event.Key() {
		case tcell.KeyEnter:
			flex.Clear()
			flex.AddItem(inputField, 1, 0, false).
				AddItem(textView, 0, 1, false).
				AddItem(footer.TextView, 1, 1, false)
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
		case 's':
			ScrollOnNewLogEntry = !ScrollOnNewLogEntry
			footer.updateLogsFooter()
		case '?':
			helpModal := modal(helpBox, 120, 30)
			pages := tview.NewPages().
				AddPage("main", flex, true, true).
				AddPage("modal", helpModal, true, true)
			pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEsc {
					pages.RemovePage("modal")
					app.SetRoot(flex, true).SetFocus(textView)
					return nil
				}
				return event
			})
			app.SetRoot(pages, true)
			return nil
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

func helpText() *tview.TextView {
	return tview.NewTextView().SetDynamicColors(true).
		SetText(`
	[orange:-:b]Navigation[white:-:B] 
	  Arrow keys
	  J / K
	  Mouse scroll
	  PageUp / PageDown
		
	[orange:-:b]Shortcuts[white:-:B] 
	  [blue:-:b]ESC[white:-:B]   Back
	  [blue:-:b]ENTER[white:-:B] Search
	  [blue:-:b]A[white:-:B]	    Attributes
	  [blue:-:b]E[white:-:B]     Environment
	  [blue:-:b]V[white:-:B]     Shell

	[orange:-:b]Modes[white:-:B] 
	  [blue:-:b]S[white:-:B]   Toggle scrolling when new log entry is added.
	`,
		)
}
