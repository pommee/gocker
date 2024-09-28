package config

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Theme struct {
	Footer struct {
		Hint       string `yaml:"hint"`
		Text       string `yaml:"text"`
		Background string `yaml:"background"`
	} `yaml:"footer"`
	Table struct {
		Fg       string `yaml:"fg"`
		Selected string `yaml:"selected"`
		Headers  string `yaml:"headers"`
	} `yaml:"table"`
}

func LoadTheme() *Theme {
	home, _ := os.UserHomeDir()
	themePath := filepath.Join(home, ".config", "gocker", "theming", "default.yml")
	data, err := os.ReadFile(themePath)
	if err != nil {
		log.Printf("error reading file: %v", err)
	}

	var theme Theme
	err = yaml.Unmarshal(data, &theme)
	if err != nil {
		log.Printf("error unmarshalling YAML: %v", err)
	}

	return &theme
}
