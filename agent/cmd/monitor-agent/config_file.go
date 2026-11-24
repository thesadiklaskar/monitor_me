package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadConfigFileArgument loads a private KEY=VALUE file when the process is
// launched as a Windows Service. The file is passed as a command-line argument
// so the Service Control Manager does not need machine-wide secret variables.
func loadConfigFileArgument() error {
	var configPath string
	for index := 1; index < len(os.Args); index++ {
		if os.Args[index] != "--config-file" {
			continue
		}
		if index+1 >= len(os.Args) {
			return fmt.Errorf("--config-file requires a path")
		}
		configPath = os.Args[index+1]
		break
	}
	if configPath == "" {
		return nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("open agent config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid agent config line %d", lineNumber)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || strings.ContainsAny(key, " \t") {
			return fmt.Errorf("invalid agent config key on line %d", lineNumber)
		}
		if strings.HasPrefix(value, "\"") {
			decoded, decodeErr := strconv.Unquote(value)
			if decodeErr != nil {
				return fmt.Errorf("invalid quoted agent config value on line %d", lineNumber)
			}
			value = decoded
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set agent config key on line %d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read agent config file: %w", err)
	}
	return nil
}
