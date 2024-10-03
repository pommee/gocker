package main

import (
	"fmt"
	"log"
	"main/internal/ui"
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

var version string
var commit string
var date string

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

func help() {
	fmt.Println("@TODO")
	os.Exit(0)
}

func update() {
	fmt.Println("@TODO")
	os.Exit(0)
}

func info() {
	blue := color.New(color.FgHiBlue).Add(color.Bold)

	printInfo := func(label, value string) {
		blue.Printf("%-10s ", label)
		fmt.Println(value)
	}

	printInfo("Version", version)
	printInfo("Commit", commit)
	printInfo("Date", date)
	os.Exit(0)
}

func main() {
	setupLogging()
	argsWithoutProg := os.Args[1:]

	for _, arg := range argsWithoutProg {
		if arg == "help" {
			help()
			break
		}
		if arg == "update" {
			update()
			break
		}
		if arg == "info" {
			info()
			break
		}
	}

	ui.Start()
}
