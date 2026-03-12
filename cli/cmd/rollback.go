package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newRollbackCmd() *cobra.Command {
	var (
		environment string
		version     string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "rollback [project]",
		Short: "Rollback to a previous deployment version",
		Long:  "Rollback a deployment to the previous or specified version",
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

			if !force {
				fmt.Printf("Rolling back %s (%s)", projectID, environment)
				if version != "" {
					fmt.Printf(" to version %s", version)
				} else {
					fmt.Print(" to previous version")
				}
				fmt.Println()
				fmt.Print("Continue? [y/N] ")

				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					fmt.Println("Rollback cancelled")
					return nil
				}
			}

			payload := map[string]interface{}{
				"environment": environment,
			}
			if version != "" {
				payload["version"] = version
			}

			body, _ := json.Marshal(payload)
			url := fmt.Sprintf("%s/api/v1/projects/%s/rollback", apiURL, projectID)

			req, err := http.NewRequest("POST", url, bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("rollback request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				var errResp map[string]string
				json.NewDecoder(resp.Body).Decode(&errResp)
				return fmt.Errorf("rollback failed: %s", errResp["error"])
			}

			var result struct {
				Status    string `json:"status"`
				Version   string `json:"version"`
				Image     string `json:"image"`
				Timestamp string `json:"timestamp"`
			}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Printf("\nRollback initiated!\n")
			fmt.Printf("  Reverting to: %s\n", result.Version)
			fmt.Printf("  Image: %s\n", result.Image)
			fmt.Println("\nUse 'nexusctl status' to check progress")
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	cmd.Flags().StringVar(&version, "version", "", "Specific version to rollback to")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
