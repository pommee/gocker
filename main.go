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
	header := tview.NewTextView().SetDynamicColors(true)
	header.SetBackgroundColor(tcell.ColorBlue)
	header.SetText("[white:#215ecf:b] ? [white:blue:B] help [white:#215ecf:b] ESC [white:#215ecf:b][white:blue:B] Quit [white:#215ecf:b] J [white:blue:B] Down [white:#215ecf:b] K [white:blue:B] Up")
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
		if event.Rune() == '?' {
			showHelpModal(table)
			return nil
		}
		return event
	})

	go updateContainerStats(app, table, containers)
	return table
}

func updateContainerStats(app *tview.Application, table *tview.Table, dockerContainers []types.Container) {
	containerMap := make(map[string]int)
	for i, container := range dockerContainers {
		containerMap[container.ID] = i + 1
	}

	// Perform initial update
	updateContainers(app, table, dockerContainers, containerMap)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if help_modal_visible {
				return
			}
			// Fetch updated container list and stats
			updatedContainers := dockerClient.GetContainers()

			// Update or add containers
			for _, container := range updatedContainers {
				go func() {
					row, exists := containerMap[container.ID]
					if exists {
						containerInfo, err := dockerClient.GetContainerInfo(container.ID)
						if err != nil {
							log.Printf("Error getting container info for %s: %v", container.ID, err)
							delete(containerMap, container.ID)
							app.QueueUpdateDraw(func() {
								table.RemoveRow(row)
							})
						}
						updateContainerRow(app, table, row, containerInfo)
					} else {
						containerMap[container.ID] = len(containerMap) + 1
						addContainerToTable(app, table, container)
					}
				}()
			}

			// Remove containers that no longer exist
			for id, row := range containerMap {
				found := false
				for _, container := range updatedContainers {
					if container.ID == id {
						found = true
						break
					}
				}
				if !found {
					delete(containerMap, id)
					app.QueueUpdateDraw(func() {
						table.RemoveRow(row)
					})
				}
			}
		}
	}
}

func updateContainers(app *tview.Application, table *tview.Table, dockerContainers []types.Container, containerMap map[string]int) {
	for _, container := range dockerContainers {
		go func() {
			row, exists := containerMap[container.ID]
			if exists {
				containerInfo, err := dockerClient.GetContainerInfo(container.ID)
				if err != nil {
					log.Printf("Error getting container info for %s: %v", container.ID, err)
					delete(containerMap, container.ID)
					app.QueueUpdateDraw(func() {
						table.RemoveRow(row)
					})
				}
				updateContainerRow(app, table, row, containerInfo)
			} else {
				containerMap[container.ID] = len(containerMap) + 1
				addContainerToTable(app, table, container)
			}
		}()
	}
}

func updateContainerRow(app *tview.Application, table *tview.Table, row int, containerInfo *docker.ContainerInfo) {
	app.QueueUpdateDraw(func() {
		table.SetCell(row, 0, tview.NewTableCell(containerInfo.ID))
		table.SetCell(row, 1, tview.NewTableCell(containerInfo.Name))
		table.SetCell(row, 2, tview.NewTableCell(containerInfo.Image))
		table.SetCell(row, 3, tview.NewTableCell(containerInfo.Uptime.String()))
		table.SetCell(row, 4, tview.NewTableCell(containerInfo.State))
		table.SetCell(row, 5, tview.NewTableCell(fmt.Sprintf("%.2f%%", containerInfo.CPUUsage)))
		table.SetCell(row, 6, tview.NewTableCell(fmt.Sprintf("%.2f MB", containerInfo.MemoryUsage)))
	})
}

func addContainerToTable(app *tview.Application, table *tview.Table, dockerContainer types.Container) {
	containerInfo, err := dockerClient.GetContainerInfo(dockerContainer.ID)
	if err != nil {
		log.Printf("Error getting container info for %s: %v", dockerContainer.ID, err)
		return
	}

	app.QueueUpdateDraw(func() {
		rowIndex := table.GetRowCount()
		table.SetCell(rowIndex, 0, tview.NewTableCell(containerInfo.ID))
		table.SetCell(rowIndex, 1, tview.NewTableCell(containerInfo.Name))
		table.SetCell(rowIndex, 2, tview.NewTableCell(containerInfo.Image))
		table.SetCell(rowIndex, 3, tview.NewTableCell(containerInfo.Uptime.String()))
		table.SetCell(rowIndex, 4, tview.NewTableCell(containerInfo.State))
		table.SetCell(rowIndex, 5, tview.NewTableCell(fmt.Sprintf("%.2f%%", containerInfo.CPUUsage)))
		table.SetCell(rowIndex, 6, tview.NewTableCell(fmt.Sprintf("%.2f MB", containerInfo.MemoryUsage)))
	})
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
			DrawHome()
			return nil
		}
		return event
	})

	// Resource
	table.SetCell(1, 0, createHelpCell("<s> (N/A)", "sort names"))
	table.SetCell(2, 0, createHelpCell("<r> (N/A)", "toggle running containers"))

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

func DrawLogs(table *tview.Table, containerID string) {
	textView := CreateTextView(app, table, containerID)
	inputField := tview.NewInputField()
	inputField.SetFieldTextColor(tcell.ColorWhite)
	inputField.SetPlaceholderTextColor(tcell.ColorLightGray)
	inputField.SetPlaceholder("Logs...")
	footer := tview.NewTextView().SetDynamicColors(true)
	footer.SetBackgroundColor(tcell.ColorBlue)
	footer.SetText("[white:#215ecf:b] ESC [white:blue:B] back [white:#215ecf:b] ENTER [white:blue:B] search [white:#215ecf:b] A [white:blue:B] attributes [white:#215ecf:b] I [white:blue:B] image")

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(inputField, 1, 0, false).
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
					// Adjust currentMatchIndex for correct 1-based index display
					displayIndex := len(matchingRegions) - currentMatchIndex
					regionID := matchingRegions[currentMatchIndex]
					textView.Highlight(regionID).ScrollToHighlight()
					currentMatchIndex = (currentMatchIndex - 1 + len(matchingRegions)) % len(matchingRegions)

					// Update title with correct 1-based index
					textView.SetTitle(fmt.Sprintf(" Result %d/%d ", displayIndex, len(matchingRegions)))
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
