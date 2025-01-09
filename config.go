package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server      ServerConfig     `yaml:"server"`
	Database    DatabaseConfig   `yaml:"database"`
	Identifiers IdentifierConfig `yaml:"identifiers"`
}

// ServerConfig holds server-specific configurations
type ServerConfig struct {
	Address      string        `yaml:"address"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	StaleTimeout time.Duration `yaml:"stale_timeout"`
}

// DatabaseConfig holds database-specific configurations
type DatabaseConfig struct {
	Driver     string `yaml:"driver"`
	Datasource string `yaml:"datasource"`
}

// IdentifierConfig holds identifier patterns
type IdentifierConfig struct {
	Patterns []string `yaml:"patterns"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ExpandIdentifiers processes patterns with nested ranges
func ExpandIdentifiers(patterns []string) []string {
	var identifiers []string

	for _, pattern := range patterns {
		identifiers = append(identifiers, expandPattern(pattern)...)
	}

	return identifiers
}

// expandPattern handles a single pattern and expands all ranges recursively
func expandPattern(pattern string) []string {
	// Find the first range [x-y]
	start := strings.Index(pattern, "[")
	end := strings.Index(pattern, "]")

	// If no range is found, return the pattern as-is
	if start == -1 || end == -1 || start > end {
		return []string{pattern}
	}

	// Split the pattern into prefix, range, and suffix
	prefix := pattern[:start]
	rangePart := pattern[start+1 : end]
	suffix := pattern[end+1:]

	rangeParts := strings.Split(rangePart, "-")
	if len(rangeParts) != 2 {
		log.Printf("Invalid range format: %s", rangePart)
		return []string{pattern}
	}

	startNum, err1 := strconv.Atoi(rangeParts[0])
	endNum, err2 := strconv.Atoi(rangeParts[1])
	if err1 != nil || err2 != nil || startNum > endNum {
		log.Printf("Invalid range numbers: %s", rangePart)
		return []string{pattern}
	}

	// Expand the range
	var expanded []string
	for i := startNum; i <= endNum; i++ {
		expandedSuffixes := expandPattern(suffix)
		for _, expSuffix := range expandedSuffixes {
			expanded = append(expanded, fmt.Sprintf("%s%d%s", prefix, i, expSuffix))
		}
	}

	return expanded
}
