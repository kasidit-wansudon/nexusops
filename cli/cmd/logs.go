package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var (
		follow      bool
		tail        int
		environment string
		service     string
		since       string
		level       string
	)

	cmd := &cobra.Command{
		Use:   "logs [project]",
		Short: "Stream deployment logs",
		Long:  "View and stream real-time logs from your deployments",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := ""
			if len(args) > 0 {
				projectID = args[0]
			} else {
				var err error
				projectID, err = getProjectID()
				if err != nil {
					return err
				}
			}

			if follow {
				return streamLogs(projectID, environment, service, level)
			}
			return fetchLogs(projectID, environment, service, level, tail, since)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&tail, "tail", "n", 100, "Number of lines to show")
	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	cmd.Flags().StringVarP(&service, "service", "s", "", "Filter by service name")
	cmd.Flags().StringVar(&since, "since", "1h", "Show logs since (e.g., 1h, 30m, 2024-01-01)")
	cmd.Flags().StringVarP(&level, "level", "l", "", "Filter by log level (debug, info, warn, error)")

	return cmd
}

func fetchLogs(projectID, environment, service, level string, tail int, since string) error {
	url := fmt.Sprintf("%s/api/v1/projects/%s/logs?env=%s&tail=%d&since=%s",
		apiURL, projectID, environment, tail, since)

	if service != "" {
		url += "&service=" + service
	}
	if level != "" {
		url += "&level=" + level
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	type LogLine struct {
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
		Service   string `json:"service"`
		Message   string `json:"message"`
	}

	var logs []LogLine
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return fmt.Errorf("failed to parse logs: %w", err)
	}

	for _, log := range logs {
		printLogLine(log.Timestamp, log.Level, log.Service, log.Message)
	}

	return nil
}

func streamLogs(projectID, environment, service, level string) error {
	url := fmt.Sprintf("%s/api/v1/projects/%s/logs/stream?env=%s",
		apiURL, projectID, environment)

	if service != "" {
		url += "&service=" + service
	}
	if level != "" {
		url += "&level=" + level
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to log stream: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Streaming logs for %s (%s)...\n", projectID, environment)
	fmt.Println("Press Ctrl+C to stop")

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == ":" {
			continue
		}

		type StreamLine struct {
			Timestamp string `json:"timestamp"`
			Level     string `json:"level"`
			Service   string `json:"service"`
			Message   string `json:"message"`
		}

		var logLine StreamLine
		if err := json.Unmarshal([]byte(line), &logLine); err != nil {
			fmt.Fprintln(os.Stderr, line)
			continue
		}

		printLogLine(logLine.Timestamp, logLine.Level, logLine.Service, logLine.Message)
	}

	return scanner.Err()
}

func printLogLine(timestamp, level, service, message string) {
	levelColor := "\033[0m"
	switch level {
	case "error":
		levelColor = "\033[31m"
	case "warn", "warning":
		levelColor = "\033[33m"
	case "info":
		levelColor = "\033[36m"
	case "debug":
		levelColor = "\033[90m"
	}

	serviceStr := ""
	if service != "" {
		serviceStr = fmt.Sprintf("\033[35m[%s]\033[0m ", service)
	}

	fmt.Printf("\033[90m%s\033[0m %s%-5s\033[0m %s%s\n",
		timestamp, levelColor, level, serviceStr, message)
}
