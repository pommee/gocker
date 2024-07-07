package main

import (
	"fmt"
	"log"
	"main/docker"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	dockerClient = docker.DockerWrapper{}
	app          = tview.NewApplication()
)

func CreateTextView(app *tview.Application, containerList *tview.Table, containerID string) *tview.TextView {
	textView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetChangedFunc(func() {
		app.Draw()
	})
	textView.SetBorder(true)

	go dockerClient.ListenForNewLogs(containerID, app, textView)

	return textView
}

// func CreateHeader(app *tview.Application) *tview.TextView {
// 	header := tview.NewTextView()
// 	header.SetBackgroundColor(tcell.ColorCornflowerBlue)
// 	header.SetText("Pocker 1.0.0")

// 	return header
// }

func CreateHelper(app *tview.Application) *tview.TextView {
	header := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)

	headerValues := map[string]string{
		"ClientVersion": dockerClient.GetDockerVersion(),
		"Containers":    strconv.Itoa(len(dockerClient.GetContainers())),
		"Images":        strconv.Itoa(len(dockerClient.GetImages())),
	}
	keys := []string{"ClientVersion", "Containers", "Images"}

	maxLength := 0
	for key := range headerValues {
		if len(key) > maxLength {
			maxLength = len(key)
		}
	}

	var sb strings.Builder
	for _, key := range keys {
		value := headerValues[key]
		padding := maxLength - len(key)
		fmt.Fprintf(&sb, "[orange]%s:[white]%*s%s\n", key, padding+1, "", value)
	}

	header.SetText(sb.String())
	return header
}

func CreateFooter(app *tview.Application) *tview.TextView {
	header := tview.NewTextView()
	header.SetBackgroundColor(tcell.ColorDimGray)
	header.SetText("[?] Help | [ESC] Quit | [J] Down | [K]Up")

	return header
}

func CreateContainerList(app *tview.Application) *tview.Table {
	table := tview.NewTable().SetSelectable(true, false)
	table.SetTitle("Containers")
	table.SetBorderPadding(0, 0, 1, 1)

	headers := []string{"ID", "Container", "Image", "Uptime", "Status", "CPU", "Memory"}
	for i, header := range headers {
		table.SetCell(0, i,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorCornflowerBlue).
				SetExpansion(1).
				SetSelectable(false))
	}

	containers := dockerClient.GetContainers()

	containerIDs := make([]string, len(containers))
	for i, container := range containers {
		containerIDs[i] = container.ID
		containerInfo, err := dockerClient.GetContainerInfo(container.ID)
		if err != nil {
			log.Fatalf("Error getting container info: %v", err)
		}

		table.SetCell(i+1, 0, tview.NewTableCell(containerInfo.ID).SetTextColor(tcell.ColorWhite))
		table.SetCell(i+1, 1, tview.NewTableCell(containerInfo.Name).SetTextColor(tcell.ColorWhite))
		table.SetCell(i+1, 2, tview.NewTableCell(containerInfo.Image).SetTextColor(tcell.ColorWhite))
		table.SetCell(i+1, 3, tview.NewTableCell(containerInfo.Uptime.String()).SetTextColor(tcell.ColorWhite))
		table.SetCell(i+1, 4, tview.NewTableCell(containerInfo.State).SetTextColor(tcell.ColorWhite))
		table.SetCell(i+1, 5, tview.NewTableCell(fmt.Sprintf("%.2f%%", containerInfo.CPUUsage)).SetTextColor(tcell.ColorWhite))
		table.SetCell(i+1, 6, tview.NewTableCell(fmt.Sprintf("%.2f MB", containerInfo.MemoryUsage)).SetTextColor(tcell.ColorWhite))
	}

	table.Select(1, 0)

	table.SetSelectedFunc(func(row, column int) {
		if row > 0 {
			containerID := containerIDs[row-1]
			DrawLogs(table, containerID)
		}
	})
	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	})
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			showHelpModal(app)
			return nil
		}
		return event
	})

	return table
}

func showHelpModal(app *tview.Application) {
	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	box := tview.NewBox().
		SetBorder(true).
		SetTitle(" Help (?) ")

	pages := tview.NewPages().
		AddPage("modal", modal(box, 0, 0), true, true)

	if err := app.SetRoot(pages, true).Run(); err != nil {
		panic(err)
	}
}

func DrawLogs(table *tview.Table, containerID string) {
	textView := CreateTextView(app, table, containerID)
	inputField := tview.NewInputField()
	inputField.SetFieldTextColor(tcell.ColorWhite)
	inputField.SetPlaceholderTextColor(tcell.ColorLightGray)
	inputField.SetPlaceholder("Logs...")
	footer := tview.NewTextView().SetDynamicColors(true)
	footer.SetBackgroundColor(tcell.ColorCornflowerBlue)
	footer.SetText("[white:#215ecf:b] ESC [white:blue:B] back [white:#215ecf:b] ENTER [white:blue:B] search [white:#215ecf:b] A [white:blue:B] attributes [white:#215ecf:b] I [white:blue:B] image")

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(inputField, 1, 0, true).
		AddItem(textView, 0, 1, false).
		AddItem(footer, 1, 1, false)

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			app.SetFocus(inputField)
		}
		if event.Key() == tcell.KeyEscape {
			DrawHome()
			return nil
		}
		return nil
	})

	// Variable to keep track of current match index and matching regions
	var currentMatchIndex int
	var matchingRegions []string

	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			keyword := inputField.GetText()
			if keyword != "" {
				// If this is the first search or keyword changed, find new matches
				if len(matchingRegions) == 0 || keyword != inputField.GetText() {
					matchingRegions = searchLogs(textView, keyword)
					currentMatchIndex = len(matchingRegions) - 1
				}

				// Highlight and scroll to the previous match
				if len(matchingRegions) > 0 {
					regionID := matchingRegions[currentMatchIndex]
					textView.Highlight(regionID).ScrollToHighlight()
					currentMatchIndex = (currentMatchIndex - 1 + len(matchingRegions)) % len(matchingRegions)
				}
			}
		case tcell.KeyEscape:
			DrawLogs(table, containerID)
		}
	})

	app.SetRoot(flex, true).SetFocus(textView)
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

func DrawHome() {
	dockerClient.NewClient()
	app.EnableMouse(true)

	//header := CreateHeader(app)
	helper := CreateHelper(app)
	containerList := CreateContainerList(app)
	footer := CreateFooter(app)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		//AddItem(header, 1, 1, false).
		AddItem(helper, 4, 1, false).
		AddItem(containerList, 0, 1, true).
		AddItem(footer, 1, 1, true)

	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
}

func main() {
	DrawHome()
}
