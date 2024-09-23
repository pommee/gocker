package main

import (
	"fmt"
	"log"
	"main/docker"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	dockerClient       = docker.DockerWrapper{}
	app                = tview.NewApplication()
	help_modal_visible = false
)

func createTextView(app *tview.Application, containerID string) *tview.TextView {
	textView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetChangedFunc(func() {
		app.Draw()
	})

	go dockerClient.ListenForNewLogs(containerID, app, textView)

	return textView
}

// func CreateHeader(app *tview.Application) *tview.TextView {
// 	header := tview.NewTextView()
// 	header.SetBackgroundColor(tcell.ColorCornflowerBlue)
// 	header.SetText("Pocker 1.0.0")

// 	return header
// }

func createHelper() *tview.TextView {
	header := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)

	headerValues := map[string]string{
		"ClientVersion": dockerClient.GetDockerVersion(),
		"Containers":    strconv.Itoa(len(dockerClient.GetContainers(true))),
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

func createFooter() *tview.TextView {
	header := tview.NewTextView().SetDynamicColors(true)
	header.SetBackgroundColor(tcell.ColorBlue)
	header.SetText("[white:#215ecf:b] ? [white:blue:B] help [white:#215ecf:b] ESC [white:#215ecf:b][white:blue:B] Quit [white:#215ecf:b] 1 [white:blue:B] running [white:#215ecf:b] 2 [white:blue:B] all")
	return header
}

func createContainerList(app *tview.Application) *tview.Table {
	table := tview.NewTable().SetSelectable(true, false)
	table.SetTitle("Containers")
	table.SetBorderPadding(0, 0, 1, 1)

	headers := []string{"ID", "Container", "Image", "Uptime", "Status", "CPU", "Memory"}
	for i, header := range headers {
		table.SetCell(0, i, tview.NewTableCell(header).
			SetTextColor(tcell.ColorCornflowerBlue).
			SetExpansion(1).
			SetSelectable(false))
	}

	// Initialize with initial containers (running only)
	initialContainers := dockerClient.GetContainers(false)
	updateTableWithContainers(app, table, initialContainers)

	table.Select(1, 0)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var containerIDs []string

	table.SetSelectedFunc(func(row, column int) {
		if row > 0 && row-1 < len(containerIDs) {
			containerID := containerIDs[row-1]
			drawLogs(table, containerID)
		}
	})

	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == '?' {
			showHelpModal(table)
			return nil
		}

		if event.Rune() == '1' || event.Rune() == '2' {
			ticker.Stop()
			allContainers := event.Rune() == '2'
			ticker = time.NewTicker(2 * time.Second)
			go updateContainerStats(app, table, allContainers, ticker, &containerIDs)
		}

		return event
	})

	go updateContainerStats(app, table, false, ticker, &containerIDs)
	return table
}

func updateContainerStats(app *tview.Application, table *tview.Table, allContainers bool, ticker *time.Ticker, containerIDs *[]string) {
	containers := dockerClient.GetContainers(allContainers)
	containerMap := createContainerMap(containers)

	updateTableWithContainers(app, table, containers)
	*containerIDs = make([]string, len(containers))
	for i, container := range containers {
		(*containerIDs)[i] = container.ID
	}

	removeStaleContainers(app, table, containerMap)

	for range ticker.C {
		if help_modal_visible {
			return
		}

		updatedContainers := dockerClient.GetContainers(allContainers)
		updateTableWithContainers(app, table, updatedContainers)
		*containerIDs = make([]string, len(updatedContainers))
		for i, container := range updatedContainers {
			(*containerIDs)[i] = container.ID
		}

		removeStaleContainers(app, table, createContainerMap(updatedContainers))
	}
}

func removeStaleContainers(app *tview.Application, table *tview.Table, containerMap map[string]int) {
	existingRows := table.GetRowCount()
	for row := 1; row < existingRows; row++ {
		idCell := table.GetCell(row, 0)
		if idCell == nil {
			continue
		}
		containerID := idCell.Text
		if _, exists := containerMap[containerID]; !exists {
			app.QueueUpdateDraw(func() {
				table.RemoveRow(row)
			})
		}
	}
}

func updateTableWithContainers(app *tview.Application, table *tview.Table, containers []types.Container) {
	containerMap := createContainerMap(containers)

	for _, container := range containers {
		go func(container types.Container) {
			containerInfo, err := dockerClient.GetContainerInfo(container.ID)
			if err != nil {
				log.Printf("Error getting container info for %s: %v", container.ID, err)
				app.QueueUpdateDraw(func() {
					table.RemoveRow(containerMap[container.ID])
					delete(containerMap, container.ID)
				})
				return
			}

			row := containerMap[container.ID]
			app.QueueUpdateDraw(func() {
				updateContainerRow(table, row, containerInfo)
			})
		}(container)
	}
}

func createContainerMap(containers []types.Container) map[string]int {
	containerMap := make(map[string]int)
	for i, container := range containers {
		containerMap[container.ID] = i + 1
	}
	return containerMap
}

func updateContainerRow(table *tview.Table, row int, containerInfo *docker.ContainerInfo) {
	table.SetCell(row, 0, tview.NewTableCell(containerInfo.ID))
	table.SetCell(row, 1, tview.NewTableCell(containerInfo.Name))
	table.SetCell(row, 2, tview.NewTableCell(containerInfo.Image))
	table.SetCell(row, 3, tview.NewTableCell(containerInfo.Uptime.String()))
	table.SetCell(row, 4, tview.NewTableCell(containerInfo.State))
	table.SetCell(row, 5, tview.NewTableCell(fmt.Sprintf("%.2f%%", containerInfo.CPUUsage)))
	table.SetCell(row, 6, tview.NewTableCell(fmt.Sprintf("%.2f MB", containerInfo.MemoryUsage)))
}

func showHelpModal(table *tview.Table) {
	table.SetSelectable(false, false)
	help_modal_visible = true
	table.Clear()
	headers := []string{"Resource", "General", "Navigation"}
	for i, header := range headers {
		table.SetCell(0, i,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorCornflowerBlue).
				SetExpansion(2).
				SetSelectable(false))

	}
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == '?' {
			help_modal_visible = false
			drawHome()
			return nil
		}
		return event
	})

	// Resource
	table.SetCell(1, 0, createHelpCell("<s> (N/A)", "sort names"))
	table.SetCell(2, 0, createHelpCell("<1>", "show running containers"))
	table.SetCell(3, 0, createHelpCell("<2>", "show all containers"))

	// General
	table.SetCell(1, 1, createHelpCell("<?>", "help"))
	table.SetCell(2, 1, createHelpCell("<q>", "quit"))

	// Navigation
	table.SetCell(1, 2, createHelpCell("<j/arrow-down>", "down"))
	table.SetCell(2, 2, createHelpCell("<k/arrow-up>  ", "up"))
}

func createHelpCell(key string, helpText string) *tview.TableCell {
	return tview.NewTableCell(fmt.Sprintf("[orange]%-20s[white]%s", key, helpText))
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
			drawLogs(table, containerID)
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

func drawLogs(table *tview.Table, containerID string) {
	textView := createTextView(app, containerID)
	inputField := createLogsInputField(table, textView, containerID)

	footer := tview.NewTextView().SetDynamicColors(true)
	footer.SetBackgroundColor(tcell.GetColor("#292929"))
	footer.SetText("[orange:#292929:b] ESC[lightgray:#292929:B] back [orange:#292929:b] ENTER[lightgray:#292929:B] search [orange:#292929:b] A[lightgray:#292929:B] attributes [orange:#292929:b] I[lightgray:#292929:B] image")

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
			drawHome()
			return nil
		}
		return nil
	})

	app.SetRoot(flex, true).SetFocus(textView)
}

func drawHome() {
	dockerClient.NewClient()
	app.EnableMouse(true)

	//header := CreateHeader(app)
	helper := createHelper()
	containerList := createContainerList(app)
	footer := createFooter()

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
	drawHome()
}
