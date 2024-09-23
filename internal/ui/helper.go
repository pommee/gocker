package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"
)

func CreateHelper() *tview.TextView {
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
