package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var (
		environment string
		watch       bool
		interval    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "status [project]",
		Short: "Check deployment status",
		Long:  "Display the current deployment status for a project",
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

			if watch {
				return watchStatus(projectID, environment, interval)
			}
			return showStatus(projectID, environment)
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch status changes")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Watch interval")

	return cmd
}

type DeploymentStatus struct {
	ProjectID   string    `json:"project_id"`
	Environment string    `json:"environment"`
	Status      string    `json:"status"`
	Version     string    `json:"version"`
	Image       string    `json:"image"`
	Replicas    int       `json:"replicas"`
	Ready       int       `json:"ready"`
	UpdatedAt   time.Time `json:"updated_at"`
	Uptime      string    `json:"uptime"`
	Containers  []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Port   int    `json:"port"`
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"containers"`
	HealthCheck struct {
		Status    string    `json:"status"`
		LastCheck time.Time `json:"last_check"`
		Failures  int       `json:"failures"`
	} `json:"health_check"`
}

func showStatus(projectID, environment string) error {
	url := fmt.Sprintf("%s/api/v1/projects/%s/status?env=%s", apiURL, projectID, environment)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var status DeploymentStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to parse status: %w", err)
	}

	printStatus(&status)
	return nil
}

func watchStatus(projectID, environment string, interval time.Duration) error {
	fmt.Printf("Watching deployment status for %s (%s)...\n", projectID, environment)
	fmt.Println("Press Ctrl+C to stop")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := showStatus(projectID, environment); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	for range ticker.C {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Watching deployment status for %s (%s)...\n\n", projectID, environment)
		if err := showStatus(projectID, environment); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}

func printStatus(s *DeploymentStatus) {
	statusIcon := "●"
	statusColor := "\033[33m"
	switch s.Status {
	case "running", "healthy":
		statusColor = "\033[32m"
	case "failed", "unhealthy":
		statusColor = "\033[31m"
	case "deploying", "scaling":
		statusColor = "\033[36m"
	}

	fmt.Printf("Project:     %s\n", s.ProjectID)
	fmt.Printf("Environment: %s\n", s.Environment)
	fmt.Printf("Status:      %s%s %s\033[0m\n", statusColor, statusIcon, s.Status)
	fmt.Printf("Version:     %s\n", s.Version)
	fmt.Printf("Image:       %s\n", s.Image)
	fmt.Printf("Replicas:    %d/%d ready\n", s.Ready, s.Replicas)
	fmt.Printf("Uptime:      %s\n", s.Uptime)
	fmt.Printf("Updated:     %s\n\n", s.UpdatedAt.Format(time.RFC3339))

	if s.HealthCheck.Status != "" {
		hcColor := "\033[32m"
		if s.HealthCheck.Status != "healthy" {
			hcColor = "\033[31m"
		}
		fmt.Printf("Health Check: %s%s\033[0m (failures: %d, last: %s)\n\n",
			hcColor, s.HealthCheck.Status, s.HealthCheck.Failures,
			s.HealthCheck.LastCheck.Format(time.RFC3339))
	}

	if len(s.Containers) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "CONTAINER\tSTATUS\tPORT\tCPU\tMEMORY\n")
		for _, c := range s.Containers {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				c.Name, c.Status, c.Port, c.CPU, c.Memory)
		}
		w.Flush()
	}
}
