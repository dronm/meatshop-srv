//go:build integration

package apitest

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func loadProjectDotEnv() error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd(): %w", err)
	}

	projectRoot, err := findProjectRoot(workingDir)
	if err != nil {
		return err
	}

	envPath := filepath.Join(projectRoot, ".env")
	file, err := os.Open(envPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", envPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if err := setAPIEnvLine(scanner.Text()); err != nil {
			return fmt.Errorf("%s:%d: %w", envPath, lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", envPath, err)
	}

	return nil
}

func findProjectRoot(startDir string) (string, error) {
	dir := filepath.Clean(startDir)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat go.mod in %s: %w", dir, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root containing go.mod was not found from %s", startDir)
		}
		dir = parent
	}
}

func setAPIEnvLine(rawLine string) error {
	line := strings.TrimSpace(rawLine)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	key, value, found := strings.Cut(line, "=")
	if !found {
		return nil
	}

	key = strings.TrimSpace(key)
	key = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(key, "?"), ":"))
	if !strings.HasPrefix(key, "API_") {
		return nil
	}
	if key == "API_" {
		return fmt.Errorf("environment variable name is invalid")
	}

	if _, exists := os.LookupEnv(key); exists {
		return nil
	}

	parsedValue, err := parseEnvValue(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("parse %s: %w", key, err)
	}

	if err := os.Setenv(key, parsedValue); err != nil {
		return fmt.Errorf("os.Setenv(%s): %w", key, err)
	}

	return nil
}

func parseEnvValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}

	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return parsed, nil
	}

	return value, nil
}
