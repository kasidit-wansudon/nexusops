package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage CI/CD pipelines",
	}

	cmd.AddCommand(
		newPipelineTriggerCmd(),
		newPipelineListCmd(),
		newPipelineStatusCmd(),
		newPipelineCancelCmd(),
	)

	return cmd
}

func newPipelineTriggerCmd() *cobra.Command {
	var (
		branch    string
		configFile string
	)

	cmd := &cobra.Command{
		Use:   "trigger [project]",
		Short: "Trigger a pipeline run",
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

			config := ""
			if configFile != "" {
				data, err := os.ReadFile(configFile)
				if err != nil {
					return fmt.Errorf("failed to read config: %w", err)
				}
				config = string(data)
			}

			payload, _ := json.Marshal(map[string]string{
				"branch": branch,
				"config": config,
			})

			url := fmt.Sprintf("%s/api/v1/projects/%s/pipelines/trigger", apiURL, projectID)
			req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to trigger pipeline: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Printf("Pipeline triggered for %s\n", projectID)
			if pipelineID, ok := result["pipeline_id"].(string); ok {
				fmt.Printf("  Pipeline ID: %s\n", pipelineID)
			}
			fmt.Printf("  Branch: %s\n", branch)
			fmt.Println("\nUse 'nexusctl pipeline status' to check progress")
			return nil
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "main", "Git branch")
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Pipeline config file")
	return cmd
}

func newPipelineListCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list [project]",
		Short: "List pipeline runs",
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

			url := fmt.Sprintf("%s/api/v1/projects/%s/pipelines?limit=%d",
				apiURL, projectID, limit)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to list pipelines: %w", err)
			}
			defer resp.Body.Close()

			type PipelineRun struct {
				ID        string    `json:"id"`
				Status    string    `json:"status"`
				Branch    string    `json:"branch"`
				Duration  string    `json:"duration"`
				StartedAt time.Time `json:"started_at"`
				Commit    string    `json:"commit"`
			}

			var runs []PipelineRun
			json.NewDecoder(resp.Body).Decode(&runs)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tSTATUS\tBRANCH\tCOMMIT\tDURATION\tSTARTED\n")
			for _, r := range runs {
				commit := r.Commit
				if len(commit) > 8 {
					commit = commit[:8]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.ID[:8], r.Status, r.Branch, commit, r.Duration,
					r.StartedAt.Format("15:04:05"))
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of runs to show")
	return cmd
}

func newPipelineStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status PIPELINE_ID",
		Short: "Show pipeline run status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("%s/api/v1/pipelines/%s", apiURL, args[0])

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to get pipeline status: %w", err)
			}
			defer resp.Body.Close()

			type StepStatus struct {
				Name     string `json:"name"`
				Status   string `json:"status"`
				Duration string `json:"duration"`
				ExitCode int    `json:"exit_code"`
			}

			type PipelineDetail struct {
				ID        string       `json:"id"`
				Status    string       `json:"status"`
				Branch    string       `json:"branch"`
				Steps     []StepStatus `json:"steps"`
				StartedAt time.Time    `json:"started_at"`
				EndedAt   *time.Time   `json:"ended_at"`
				Duration  string       `json:"duration"`
			}

			var detail PipelineDetail
			json.NewDecoder(resp.Body).Decode(&detail)

			statusColor := "\033[33m"
			switch detail.Status {
			case "success":
				statusColor = "\033[32m"
			case "failed":
				statusColor = "\033[31m"
			case "running":
				statusColor = "\033[36m"
			}

			fmt.Printf("Pipeline: %s\n", detail.ID)
			fmt.Printf("Status:   %s%s\033[0m\n", statusColor, detail.Status)
			fmt.Printf("Branch:   %s\n", detail.Branch)
			fmt.Printf("Duration: %s\n\n", detail.Duration)

			fmt.Println("Steps:")
			for _, step := range detail.Steps {
				icon := "○"
				switch step.Status {
				case "success":
					icon = "\033[32m✓\033[0m"
				case "failed":
					icon = "\033[31m✗\033[0m"
				case "running":
					icon = "\033[36m◐\033[0m"
				case "skipped":
					icon = "\033[90m⊘\033[0m"
				}
				fmt.Printf("  %s %s (%s)\n", icon, step.Name, step.Duration)
			}
			return nil
		},
	}
}

func newPipelineCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel PIPELINE_ID",
		Short: "Cancel a running pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("%s/api/v1/pipelines/%s/cancel", apiURL, args[0])

			req, err := http.NewRequest("POST", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to cancel pipeline: %w", err)
			}
			resp.Body.Close()

			fmt.Printf("Pipeline %s cancelled\n", args[0])
			return nil
		},
	}
}
