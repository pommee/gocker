package ui

import (
	"context"
	"fmt"
	"log"
	"main/internal/config"
	"main/internal/docker"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app          *tview.Application
	dockerClient = docker.DockerWrapper{}
	containerMap = make(map[string]int)
	mapMutex     sync.Mutex // Mutex for synchronizing access to containerMap
	theme        = config.LoadTheme()
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
	footer := CreateFooterHome()

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(helper, 4, 1, false).
		AddItem(containerList, 0, 1, true).
		AddItem(footer, 1, 1, true)

	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
}

func createContainerList() *tview.Table {
	table := setupContainerTable()
	initialContainers := dockerClient.GetContainers(true)
	updateTableWithContainers(table, initialContainers)

	table.Select(1, 0)

	ctx, cancel := context.WithCancel(context.Background())
	eventChan := make(chan events.Message)
	startDockerEventListener(ctx, eventChan, table)
	go startContainerStatsFetching(ctx, table)

	table.SetSelectedFunc(func(row, column int) {
		handleContainerSelection(row, initialContainers, cancel, table)
	})
	table.SetDoneFunc(handleDone)
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return handleInput(event, table)
	})

	return table
}

func startContainerStatsFetching(ctx context.Context, table *tview.Table) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopped container stats fetching.")
			return
		case <-ticker.C:
			containers := dockerClient.GetContainers(true)
			updateTableWithContainers(table, containers)
		}
	}
}

func setupContainerTable() *tview.Table {
	table := tview.NewTable().SetSelectable(true, false)
	table.SetTitle("Containers")
	table.SetBorderPadding(0, 0, 1, 1)
	table.SetBackgroundColor(tcell.GetColor(theme.Table.Fg))
	table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.GetColor(theme.Table.Selected)))

	headers := []string{"ID", "Container", "Image", "Uptime", "Status", "CPU", "Memory"}
	for i, header := range headers {
		table.SetCell(0, i, tview.NewTableCell(header).
			SetTextColor(tcell.GetColor(theme.Table.Headers)).
			SetExpansion(1).
			SetSelectable(false))
	}

	return table
}

func startDockerEventListener(ctx context.Context, eventChan chan events.Message, table *tview.Table) {
	go dockerClient.ListenForEvents(ctx, eventChan)

	go func() {
		for event := range eventChan {
			handleDockerEvent(event, table)
		}
	}()
}

func handleContainerSelection(row int, containers []types.Container, cancel context.CancelFunc, table *tview.Table) {
	containerID := containers[row-1].ID
	cancel() // Cancel the fetching goroutine
	DrawLogs(table, containerID)
}

func handleDone(key tcell.Key) {
	if key == tcell.KeyEscape {
		app.Stop()
	}
}

func handleInput(event *tcell.EventKey, table *tview.Table) *tcell.EventKey {
	if event.Rune() == '?' {
		showHelpModal(table)
		return nil
	}
	return event
}

func handleDockerEvent(event events.Message, table *tview.Table) {
	log.Printf("[event] Action: %s, ID: %s, Status: %s", event.Action, event.ID, event.Status)

	// Refresh the table when containers start or stop
	if event.Action == "start" || event.Action == "stop" {
		app.QueueUpdateDraw(func() {
			containers := dockerClient.GetContainers(true)
			updateTableWithContainers(table, containers)
		})
	}

	if event.Action == "destroy" {
		app.QueueUpdateDraw(func() {
			mapMutex.Lock() // Lock the map for safe access
			defer mapMutex.Unlock()
			if row, exists := containerMap[event.ID]; exists {
				table.RemoveRow(row)
				delete(containerMap, event.ID)
			}
		})
	}
}

func createHelpCell(key string, helpText string) *tview.TableCell {
	return tview.NewTableCell(fmt.Sprintf("[orange]%-20s[white]%s", key, helpText))
}

func showHelpModal(table *tview.Table) {
	table.SetSelectable(false, false)
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

func updateTableWithContainers(table *tview.Table, containers []types.Container) {
	// Clear previous mappings
	mapMutex.Lock()
	defer mapMutex.Unlock() // Ensure we unlock after updating

	containerMap = make(map[string]int)

	for rowIndex, container := range containers {
		go func(container types.Container, row int) {
			containerInfo, err := dockerClient.GetContainerInfo(container.ID)
			if err != nil {
				log.Printf("Error getting container info for %s: %v", container.ID, err)
				app.QueueUpdateDraw(func() {
					mapMutex.Lock() // Lock the map during access
					defer mapMutex.Unlock()
					table.RemoveRow(row)
					delete(containerMap, container.ID)
				})
				return
			}

			mapMutex.Lock()
			containerMap[container.ID] = row
			mapMutex.Unlock()

			app.QueueUpdateDraw(func() {
				updateContainerRow(table, row, containerInfo)
			})
		}(container, rowIndex+1)
	}
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
