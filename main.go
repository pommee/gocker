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

func CreateHeader(app *tview.Application) *tview.TextView {
	header := tview.NewTextView()
	header.SetBackgroundColor(tcell.ColorCornflowerBlue)
	header.SetText("Pocker 1.0.0")

	return header
}

func CreateHelper(app *tview.Application) *tview.TextView {
	header := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	header.SetBorderPadding(1, 1, 0, 0)

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
	table.SetBorderPadding(0, 1, 1, 1)

	headers := []string{"ID", "Container", "Image", "Uptime", "Status", "CPU", "Memory"}
	for i, header := range headers {
		table.SetCell(0, i,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorCornflowerBlue).
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

		idCell := tview.NewTableCell(containerInfo.ID).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1)

		nameCell := tview.NewTableCell(containerInfo.Name).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(2)

		imageCell := tview.NewTableCell(containerInfo.Image).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1)

		uptimeCell := tview.NewTableCell(containerInfo.Uptime.String()).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1)

		statusCell := tview.NewTableCell(containerInfo.State).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1)

		cpuCell := tview.NewTableCell(fmt.Sprintf("%.2f%%", containerInfo.CPUUsage)).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1)

		memoryCell := tview.NewTableCell(fmt.Sprintf("%.2f MB", containerInfo.MemoryUsage)).
			SetTextColor(tcell.ColorWhite)

		table.SetCell(i+1, 0, idCell)
		table.SetCell(i+1, 1, nameCell)
		table.SetCell(i+1, 2, imageCell)
		table.SetCell(i+1, 3, uptimeCell)
		table.SetCell(i+1, 4, statusCell)
		table.SetCell(i+1, 5, cpuCell)
		table.SetCell(i+1, 6, memoryCell)

		table.SetSelectedFunc(func(row, column int) {
			if row > 0 {
				containerID := containerIDs[row-1]
				DrawLogs(table, containerID)
			}
		})
	}

	table.Select(1, 0)
	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	})
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
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
			return nil
		}
		return event
	})

	return table
}

func DrawLogs(table *tview.Table, containerID string) {
	textView := CreateTextView(app, table, containerID)
	inputField := tview.NewInputField()

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

	inputField.SetPlaceholder("Logs...")

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
					currentMatchIndex = 0
				}

				// Highlight and scroll to the next match
				if len(matchingRegions) > 0 {
					regionID := matchingRegions[currentMatchIndex]
					textView.Highlight(regionID).ScrollToHighlight()
					currentMatchIndex = (currentMatchIndex + 1) % len(matchingRegions)
				}
			}
		case tcell.KeyEscape:
			DrawLogs(table, containerID)
		}
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(inputField, 1, 0, true).
		AddItem(textView, 0, 1, false)

	app.SetRoot(flex, true).SetFocus(textView)
}

func searchLogs(textView *tview.TextView, keyword string) []string {
	text := textView.GetText(true)
	lines := strings.Split(text, "\n")
	var matchingRegions []string
	var highlightedText strings.Builder

	for index, line := range lines {
		if strings.Contains(line, keyword) {
			parts := strings.Split(line, keyword)
			// Construct the line with the matched word highlighted
			highlightedLine := fmt.Sprintf("%s[orange:black]%s[gray:black]%s\n", parts[0], keyword, parts[1])
			regionID := fmt.Sprintf("match%d", index)
			highlightedText.WriteString(fmt.Sprintf(`["%s"]%s[""]`, regionID, highlightedLine))
			matchingRegions = append(matchingRegions, regionID)
		} else {
			// Use gray color for non-matching lines
			highlightedText.WriteString(fmt.Sprintf("[gray:black]%s\n", line))
		}
	}

	textView.SetText(highlightedText.String())
	return matchingRegions
}

func DrawHome() {
	dockerClient.NewClient()
	app.EnableMouse(true)

	header := CreateHeader(app)
	helper := CreateHelper(app)
	containerList := CreateContainerList(app)
	footer := CreateFooter(app)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 1, false).
		AddItem(helper, 5, 1, false).
		AddItem(containerList, 0, 1, true).
		AddItem(footer, 1, 1, true)

	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
}

func main() {
	DrawHome()
}
