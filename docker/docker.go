package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/rivo/tview"
)

type DockerImage struct {
	ID         string
	Repository []string
	Size       int64
}

type DockerWrapper struct {
	client *client.Client
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

func (dc *DockerWrapper) NewClient() {
	var err error
	dc.client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err.Error())
	}
}

func (dc *DockerWrapper) CloseClient() {
	dc.client.Close()
}

func (dc *DockerWrapper) GetContainers() []types.Container {
	containers, err := dc.client.ContainerList(context.Background(), container.ListOptions{All: true})
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

	startedAt := container.State.StartedAt
	startTime, err := time.Parse("2006-01-02T15:04:05.999999999Z", startedAt)
	if err != nil {
		return nil, err
	}

	running := container.State.Running
	var uptime time.Duration
	if running {
		uptime = time.Since(startTime).Round(time.Second)
	}

	return &ContainerInfo{
		ID:          container.ID[:12],
		Name:        container.Name,
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

func (dc *DockerWrapper) ListenForNewLogs(id string, app *tview.Application, textView *tview.TextView) {
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	out, err := dc.client.ContainerLogs(context.Background(), id, logOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching container logs: %v\n", err)
		return
	}
	defer out.Close()

	var buffer bytes.Buffer
	header := make([]byte, 8)
	for {
		_, err := io.ReadFull(out, header)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading log header: %v\n", err)
			return
		}

		/*
			This is due to multiplexing.
			Byte 1: Stream indicator (0x01 for stdout, 0x02 for stderr).
			Bytes 2-4: Unused (set to 0x00).
			Bytes 5-8: Big-endian 32-bit integer representing the length of the log message.
		*/
		logLength := int64(header[4])<<24 | int64(header[5])<<16 | int64(header[6])<<8 | int64(header[7])
		logMessage := make([]byte, logLength)
		_, err = io.ReadFull(out, logMessage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading log message: %v\n", err)
			return
		}

		buffer.Write(logMessage)
	}

	var highlightedBuffer bytes.Buffer
	err = quick.Highlight(&highlightedBuffer, buffer.String(), "Docker", "terminal16m", "monokai")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error highlighting log content: %v\n", err)
		highlightedBuffer.Write(buffer.Bytes())
	}

	fmt.Fprint(tview.ANSIWriter(textView), highlightedBuffer.String())
	textView.ScrollToEnd()
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

func (dc *DockerWrapper) StartContainer(id string) {
	dc.client.ContainerStart(context.Background(), id, container.StartOptions{})
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
