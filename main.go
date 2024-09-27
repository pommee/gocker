package main

import (
	"log"
	"main/internal/ui"
	"os"
	"path/filepath"
)

func setupLogging() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}

	logFilePath := filepath.Join(homeDir, ".config", "gocker", "app.log")
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(file)
}

func main() {
	setupLogging()
	ui.Start()
}
