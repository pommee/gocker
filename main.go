package main

import (
	"encoding/json"
	"fmt"
	"log"
	"main/internal/ui"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var version, commit, date string

type Release struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
}

type Change struct {
	Message string
	Commit  string
}

func logFatal(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func setupLogging() {
	homeDir, err := os.UserHomeDir()
	logFatal(err, "Failed to get home directory")

	logFilePath := filepath.Join(homeDir, ".config", "gocker", "app.log")
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	logFatal(err, "Failed to open log file")

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(file)
}

func fetchURL(url string) *http.Response {
	resp, err := http.Get(url)
	logFatal(err, "Failed to make HTTP request")
	return resp
}

func getLatestTag() string {
	resp := fetchURL("https://api.github.com/repos/pommee/gocker/releases/latest")
	defer resp.Body.Close()

	var release Release
	err := json.NewDecoder(resp.Body).Decode(&release)
	logFatal(err, "Failed to decode JSON")

	return strings.TrimPrefix(release.TagName, "v")
}

func runCommand(command string) (bool, error) {
	cmd := exec.Command("sh", "-c", command)
	_, err := cmd.CombinedOutput()
	return err == nil, err
}

func update() {
	success, err := runCommand("curl https://raw.githubusercontent.com/pommee/gocker/main/installer.sh | sh /dev/stdin")
	if success {
		color.New(color.FgGreen, color.Bold).Println("Successfully updated!")
	} else {
		fmt.Println("Update failed:", err)
	}
}

func validateVersion() {
	current, _ := semver.NewVersion(version)
	latest, err := semver.NewVersion(getLatestTag())
	logFatal(err, "Failed to parse latest version")

	switch current.Compare(latest) {
	case -1:
		fmt.Printf("Updating from %s to %s\n", version, latest)
		update()
	case 0:
		color.New(color.FgHiBlue, color.Bold).Printf("Running latest! [%s]\n", version)
	}
	os.Exit(0)
}

func info() {
	changes := parseChanges()
	for _, change := range changes {
		fmt.Println(change)
		clickableCommit(change.Commit, change.Message)
	}
	blue := color.New(color.FgHiBlue).Add(color.Bold)
	blue.Printf("%-10s %s\n", "Version", version)
	blue.Printf("%-10s %s\n", "Commit", commit)
	blue.Printf("%-10s %s\n", "Date", date)
	os.Exit(0)
}

func clickableCommit(hash string, message string) {
	commitURL := "https://github.com/pommee/gocker/commit/" + hash
	fmt.Printf("\033]8;;%s\033\\%s\033]8;;\033\\ %s\n", commitURL, hash, message)
}

func parseChanges() []Change {
	resp, err := http.Get("https://api.github.com/repos/pommee/gocker/releases")
	if err != nil {
		log.Fatalf("Error fetching releases: %v", err)
	}
	defer resp.Body.Close()

	var releases []Release
	var changes []Change

	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	changeRegex := regexp.MustCompile(`(.+?) \((\w+)\)`)
	re := regexp.MustCompile(`(?s)##.*?\n\n\*`)

	for _, release := range releases {
		body := re.ReplaceAllString(release.Body, "")
		body = regexp.MustCompile(`\*`).ReplaceAllString(body, "")

		changeMatches := changeRegex.FindAllStringSubmatch(body, -1)

		for _, match := range changeMatches {
			changes = append(changes, Change{Message: strings.TrimSpace(match[1]), Commit: strings.TrimSpace(match[2])})
		}
	}

	return changes
}

func main() {
	setupLogging()

	rootCmd := &cobra.Command{
		Use:   "gocker",
		Short: "Gocker - A TUI Tool for Docker Management",
		Run: func(cmd *cobra.Command, args []string) {
			ui.Start()
		},
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Update to the latest version",
		Run: func(cmd *cobra.Command, args []string) {
			validateVersion()
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show gocker info",
		Run: func(cmd *cobra.Command, args []string) {
			info()
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
