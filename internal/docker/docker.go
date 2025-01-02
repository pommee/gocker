package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"main/internal/config"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

type DockerImage struct {
	ID         string
	Repository []string
	Size       int64
}

type DockerWrapper struct {
	client          *client.Client
	IsClientCreated bool
}

type ContainerInfo struct {
	ID          string
	Name        string
	CPUUsage    float64
	MemoryUsage float64
	State       string
	Uptime      time.Duration
	Image       string
}

type LogsSettings struct {
	initialAmountOfLogs string
}

var logsSettings LogsSettings

func (dc *DockerWrapper) NewClient(config config.Config) {
	var err error
	dc.client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err.Error())
	}
	dc.IsClientCreated = true
	logsSettings = LogsSettings{
		initialAmountOfLogs: config.InitialAmountOfLogs,
	}
}

func (dc *DockerWrapper) CloseClient() {
	dc.client.Close()
	dc.IsClientCreated = false
}

func (dc *DockerWrapper) GetContainers(allContainers bool) []types.Container {
	containers, err := dc.client.ContainerList(context.Background(), container.ListOptions{All: allContainers})
	if err != nil {
		panic(err)
	}
	return containers
}

func (dc *DockerWrapper) GetImages() []image.Summary {
	images, err := dc.client.ImageList(
		context.Background(),
		image.ListOptions{},
	)
	if err != nil {
		panic(err)
	}
	return images
}

func (dc *DockerWrapper) GetDockerVersion() string {
	ver, _ := dc.client.ServerVersion(context.Background())
	return ver.Version
}

func (dc *DockerWrapper) GetDockerVolumes() []*volume.Volume {
	dockerVolumes, _ := dc.client.VolumeList(context.Background(), volume.ListOptions{})
	return dockerVolumes.Volumes
}

func (dc *DockerWrapper) GetAttributes(containerID string) string {
	containerInfo, _ := dc.client.ContainerInspect(context.Background(), containerID)
	infoJSON, _ := json.MarshalIndent(containerInfo, "", "  ")
	return string(infoJSON)
}

func (dc *DockerWrapper) GetEnvironmentVariables(containerID string) string {
	containerInfo, _ := dc.client.ContainerInspect(context.Background(), containerID)
	envVars, _ := json.MarshalIndent(containerInfo.Config.Env, "", "  ")
	return string(envVars)
}

func (dc *DockerWrapper) ListenForEvents(ctx context.Context, eventChan chan<- events.Message) {
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", "container")

	messages, errs := dc.client.Events(ctx, events.ListOptions{
		Filters: eventFilter,
	})

	for {
		select {
		case event := <-messages:
			eventChan <- event
		case err := <-errs:
			if err != nil {
				log.Printf("Error while listening to Docker events: %v", err)
				close(eventChan)
				return
			}
		case <-ctx.Done():
			close(eventChan)
			return
		}
	}
}

func (dc *DockerWrapper) CreateContainerShell(ctx context.Context, containerID string, textView *tview.TextView) error {
	commands := []string{
		"/bin/ash",
		"/bin/bash",
	}

	var shell types.IDResponse
	for _, command := range commands {
		execConfig := container.ExecOptions{
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Tty:          true,
			Cmd:          []string{command},
		}
		resp, err := dc.client.ContainerExecCreate(ctx, containerID, execConfig)
		if err == nil {
			shell = resp
			break
		}
	}

	attachResp, err := dc.client.ContainerExecAttach(ctx, shell.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return err
	}

	err = dc.client.ContainerExecStart(ctx, shell.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return err
	}

	inputReader, inputWriter := io.Pipe()
	dc.updateTerminalSize(ctx, shell.ID)

	go func() {
		io.Copy(tview.ANSIWriter(textView), attachResp.Reader)
		textView.ScrollToEnd()
	}()

	go func() {
		io.Copy(attachResp.Conn, inputReader)
	}()

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			inputWriter.Write([]byte("\n"))
		} else if event.Key() == tcell.KeyCtrlC {
			return event
		} else {
			inputWriter.Write([]byte(string(event.Rune())))
		}
		return nil
	})

	return nil
}

func (dc *DockerWrapper) updateTerminalSize(ctx context.Context, execID string) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return
	}

	dc.client.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: uint(height),
		Width:  uint(width),
	})
}
func (dc *DockerWrapper) GetContainerInfo(id string) (*ContainerInfo, error) {
	container, err := dc.client.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	stats, err := dc.client.ContainerStatsOneShot(context.Background(), id)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	var statsJSON types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&statsJSON); err != nil {
		return nil, err
	}

	cpuUsage := calculateCPUUsage(&statsJSON)
	memoryUsage := calculateMemoryUsageMb(&statsJSON)

	startTime, err := time.Parse(time.RFC3339Nano, container.State.StartedAt)
	if err != nil {
		return nil, err
	}

	uptime := time.Since(startTime).Round(time.Second)

	return &ContainerInfo{
		ID:          container.ID[:12],
		Name:        strings.TrimPrefix(container.Name, "/"),
		CPUUsage:    cpuUsage,
		MemoryUsage: memoryUsage,
		State:       container.State.Status,
		Uptime:      uptime,
		Image:       container.Config.Image,
	}, nil
}

func calculateCPUUsage(stats *types.StatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

func calculateMemoryUsageMb(stats *types.StatsJSON) float64 {
	return float64(stats.MemoryStats.Usage) / float64(1024*1024)
}

func (dc *DockerWrapper) ListenForNewLogs(ctx context.Context, id string, app *tview.Application, textView *tview.TextView, scrollOnNewLogEntry *bool) {
	initialLogs, err := dc.fetchContainerLogs(id, false, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching container logs: %v\n", err)
		return
	}

	const (
		chunkSize    = 60_000
		maxDisplayed = 10_000
	)

	type processedChunk struct {
		index int
		data  string
	}

	numChunks := (len(initialLogs) + chunkSize - 1) / chunkSize
	chunkChan := make(chan processedChunk, numChunks)
	chunks := make([]processedChunk, numChunks)

	var wg sync.WaitGroup
	wg.Add(numChunks)

	// Process logs in chunks
	for i := 0; i < numChunks; i++ {
		go func(index int) {
			defer wg.Done()
			start := index * chunkSize
			end := (index + 1) * chunkSize
			if end > len(initialLogs) {
				end = len(initialLogs)
			}
			chunk := initialLogs[start:end]
			highlightedChunk := dc.highlightLogs(chunk)
			chunkChan <- processedChunk{index: index, data: highlightedChunk}
		}(i)
	}

	// Collect processed chunks
	go func() {
		for chunk := range chunkChan {
			chunks[chunk.index] = chunk
		}
	}()

	go func() {
		wg.Wait()
		close(chunkChan)
	}()

	wg.Wait()

	var sb strings.Builder
	sb.Grow(len(initialLogs) * 100)
	for _, chunk := range chunks {
		sb.WriteString(chunk.data)
	}

	allLogs := sb.String()
	lastLogs := getLastNLines(allLogs, maxDisplayed)

	app.QueueUpdateDraw(func() {
		textView.Clear()
		fmt.Fprint(tview.ANSIWriter(textView), lastLogs)
		textView.ScrollToEnd()
	})

	liveLogs, err := dc.startLogStream(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching live logs: %v\n", err)
		return
	}
	defer liveLogs.Close()

	dc.streamLogs(ctx, liveLogs, app, textView, scrollOnNewLogEntry)
}

func getLastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		return strings.Join(lines[len(lines)-n:], "\n")
	}
	return s
}

func (dc *DockerWrapper) fetchContainerLogs(id string, follow bool, since string) (string, error) {
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Since:      since,
		Tail:       logsSettings.initialAmountOfLogs,
	}

	out, err := dc.client.ContainerLogs(context.Background(), id, logOptions)
	if err != nil {
		return "", err
	}
	defer out.Close()

	var logBuffer bytes.Buffer
	header := make([]byte, 8)
	for {
		_, err := io.ReadFull(out, header)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("error reading log header: %v", err)
		}

		logMessage, err := dc.readLogMessage(out, header)
		if err != nil {
			return "", err
		}

		logBuffer.Write(logMessage)
	}

	return logBuffer.String(), nil
}

func (dc *DockerWrapper) startLogStream(id string) (io.ReadCloser, error) {
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Since:      time.Now().Format(time.RFC3339),
	}

	out, err := dc.client.ContainerLogs(context.Background(), id, logOptions)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (dc *DockerWrapper) streamLogs(ctx context.Context, out io.ReadCloser, app *tview.Application, textView *tview.TextView, scrollOnNewLogEntry *bool) {
	header := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, err := io.ReadFull(out, header)
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Fprintf(os.Stderr, "Error reading log header: %v\n", err)
				return
			}

			logMessage, err := dc.readLogMessage(out, header)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading log message: %v\n", err)
				return
			}

			highlightedLog := dc.highlightLogs(string(logMessage))
			app.QueueUpdateDraw(func() {
				fmt.Fprint(tview.ANSIWriter(textView), highlightedLog)
				if *scrollOnNewLogEntry {
					textView.ScrollToEnd()
				}
			})
		}
	}
}

func (dc *DockerWrapper) readLogMessage(out io.Reader, header []byte) ([]byte, error) {
	/*
		This is due to multiplexing.
		Byte 1: Stream indicator (0x01 for stdout, 0x02 for stderr).
		Bytes 2-4: Unused (set to 0x00).
		Bytes 5-8: Big-endian 32-bit integer representing the length of the log message.
	*/
	logLength := int64(header[4])<<24 | int64(header[5])<<16 | int64(header[6])<<8 | int64(header[7])
	logMessage := make([]byte, logLength)
	_, err := io.ReadFull(out, logMessage)
	if err != nil {
		return nil, fmt.Errorf("error reading log message: %v", err)
	}
	return logMessage, nil
}

func (dc *DockerWrapper) highlightLogs(logs string) string {
	var highlightedBuffer bytes.Buffer
	err := quick.Highlight(&highlightedBuffer, logs, "Docker", "terminal16m", "monokai")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error highlighting log content: %v\n", err)
		highlightedBuffer.WriteString(logs)
	}
	return highlightedBuffer.String()
}

func (dc *DockerWrapper) PauseContainer(id string) {
	dc.client.ContainerPause(context.Background(), id)
}

func (dc *DockerWrapper) PauseContainers(ids []string) {
	for _, id := range ids {
		dc.PauseContainer(id)
	}
}

func (dc *DockerWrapper) UnpauseContainer(id string) {
	dc.client.ContainerUnpause(context.Background(), id)
}

func (dc *DockerWrapper) UnpauseContainers(ids []string) {
	for _, id := range ids {
		dc.UnpauseContainer(id)
	}
}

func (dc *DockerWrapper) StartContainer(id string) error {
	err := dc.client.ContainerStart(context.Background(), id, container.StartOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (dc *DockerWrapper) StartContainers(ids []string) {
	for _, id := range ids {
		dc.StartContainer(id)
	}
}

func (dc *DockerWrapper) StopContainer(id string) {
	dc.client.ContainerStop(context.Background(), id, container.StopOptions{})
}

func (dc *DockerWrapper) StopContainers(ids []string) {
	for _, id := range ids {
		dc.StopContainer(id)
	}
}

func (dc *DockerWrapper) RemoveContainer(id string) error {
	err := dc.client.ContainerRemove(context.Background(), id, container.RemoveOptions{})
	if err != nil {
		return err
	}
	return nil
}
