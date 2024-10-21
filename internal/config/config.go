package config

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var (
	home, _    = os.UserHomeDir()
	configPath = filepath.Join(home, ".config", "gocker", "config", "config.yml")
	themePath  = filepath.Join(home, ".config", "gocker", "theming", "default.yml")
)

type Config struct {
	OnlyRunningOnStartup bool `yaml:"onlyRunningOnStartup"`
}

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

func LoadConfig() *Config {
	data, err := os.ReadFile(configPath)
	log.Println(data)
	if err != nil {
		log.Printf("error reading file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("error unmarshalling YAML: %v", err)
	}

	return &config
}

func LoadTheme() *Theme {
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
