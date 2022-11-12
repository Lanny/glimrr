package main

import (
	"encoding/json"
	gloss "github.com/charmbracelet/lipgloss"
	"os"
)

type GLIMRRFileConfigColors struct {
	Background string
}

type GLIMRRFileConfig struct {
	Colors GLIMRRFileConfigColors
}

type GLIMRRConfigColors struct {
	Background gloss.Color
}

type GLIMRRConfig struct {
	Colors GLIMRRConfigColors
}

func fileConfigToConfig(f GLIMRRFileConfig) *GLIMRRConfig {
	return &GLIMRRConfig{
		Colors: GLIMRRConfigColors{
			Background: gloss.Color(f.Colors.Background),
		},
	}
}

func loadConfigFromFile(path string) (*GLIMRRConfig, error) {
	config := defaultFileConfig

	data, err := os.ReadFile(path)
	if nil != err {
		return fileConfigToConfig(defaultFileConfig), err
	}

	err = json.Unmarshal(data, &config)
	if nil != err {
		return fileConfigToConfig(defaultFileConfig), err
	}

	return fileConfigToConfig(config), nil
}

var defaultFileConfig = GLIMRRFileConfig{
	Colors: GLIMRRFileConfigColors{
		Background: "#000",
	},
}

var CFG = fileConfigToConfig(defaultFileConfig)
