package ui

import (
	"fmt"
	"log"
	"main/internal/docker"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app                *tview.Application
	dockerClient       = docker.DockerWrapper{}
	help_modal_visible = false
)

func Start() {
	app = tview.NewApplication()
	dockerClient.NewClient()
	DrawHome()
}

func DrawHome() {
	app.EnableMouse(true)

	helper := CreateHelper()
	containerList := createContainerList()
	footer := CreateFooter()

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(helper, 4, 1, false).
		AddItem(containerList, 0, 1, true).
		AddItem(footer, 1, 1, true)

	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
}

func createContainerList() *tview.Table {
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
	updateTableWithContainers(table, initialContainers)

	table.Select(1, 0)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var containerIDs []string

	table.SetSelectedFunc(func(row, column int) {
		if row > 0 && row-1 < len(containerIDs) {
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

		if event.Rune() == '1' || event.Rune() == '2' {
			ticker.Stop()
			allContainers := event.Rune() == '2'
			ticker = time.NewTicker(2 * time.Second)
			go updateContainerStats(table, allContainers, ticker, &containerIDs)
		}

		return event
	})

	go updateContainerStats(table, false, ticker, &containerIDs)
	return table
}

func createHelpCell(key string, helpText string) *tview.TableCell {
	return tview.NewTableCell(fmt.Sprintf("[orange]%-20s[white]%s", key, helpText))
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
	table.SetCell(2, 0, createHelpCell("<1>", "show running containers"))
	table.SetCell(3, 0, createHelpCell("<2>", "show all containers"))

	// General
	table.SetCell(1, 1, createHelpCell("<?>", "help"))
	table.SetCell(2, 1, createHelpCell("<q>", "quit"))

	// Navigation
	table.SetCell(1, 2, createHelpCell("<j/arrow-down>", "down"))
	table.SetCell(2, 2, createHelpCell("<k/arrow-up>  ", "up"))
}

func removeStaleContainers(table *tview.Table, containerMap map[string]int) {
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

func updateContainerStats(table *tview.Table, allContainers bool, ticker *time.Ticker, containerIDs *[]string) {
	containers := dockerClient.GetContainers(allContainers)
	containerMap := createContainerMap(containers)

	updateTableWithContainers(table, containers)
	*containerIDs = make([]string, len(containers))
	for i, container := range containers {
		(*containerIDs)[i] = container.ID
	}

	removeStaleContainers(table, containerMap)

	for range ticker.C {
		if help_modal_visible {
			return
		}

		updatedContainers := dockerClient.GetContainers(allContainers)
		updateTableWithContainers(table, updatedContainers)
		*containerIDs = make([]string, len(updatedContainers))
		for i, container := range updatedContainers {
			(*containerIDs)[i] = container.ID
		}

		removeStaleContainers(table, createContainerMap(updatedContainers))
	}
}

func updateTableWithContainers(table *tview.Table, containers []types.Container) {
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
