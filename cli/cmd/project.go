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

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}

	cmd.AddCommand(
		newProjectListCmd(),
		newProjectCreateCmd(),
		newProjectInfoCmd(),
		newProjectDeleteCmd(),
	)

	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := http.NewRequest("GET", apiURL+"/api/v1/projects", nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to list projects: %w", err)
			}
			defer resp.Body.Close()

			type Project struct {
				ID          string    `json:"id"`
				Name        string    `json:"name"`
				Repository  string    `json:"repository"`
				Status      string    `json:"status"`
				Environment string    `json:"environment"`
				UpdatedAt   time.Time `json:"updated_at"`
			}

			var projects []Project
			json.NewDecoder(resp.Body).Decode(&projects)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "NAME\tSTATUS\tREPOSITORY\tENVIRONMENT\tUPDATED\n")
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.Name, p.Status, p.Repository, p.Environment,
					p.UpdatedAt.Format("2006-01-02 15:04"))
			}
			w.Flush()
			return nil
		},
	}
}

func newProjectCreateCmd() *cobra.Command {
	var (
		repository  string
		description string
		teamID      string
	)

	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a new project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, _ := json.Marshal(map[string]string{
				"name":        args[0],
				"repository":  repository,
				"description": description,
				"team_id":     teamID,
			})

			req, err := http.NewRequest("POST", apiURL+"/api/v1/projects",
				bytes.NewReader(payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to create project: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				var errResp map[string]string
				json.NewDecoder(resp.Body).Decode(&errResp)
				return fmt.Errorf("failed to create project: %s", errResp["error"])
			}

			fmt.Printf("Project '%s' created successfully\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repo", "r", "", "Git repository URL (required)")
	cmd.Flags().StringVarP(&description, "desc", "d", "", "Project description")
	cmd.Flags().StringVarP(&teamID, "team", "t", "", "Team ID")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func newProjectInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [project]",
		Short: "Show project details",
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

			req, err := http.NewRequest("GET",
				fmt.Sprintf("%s/api/v1/projects/%s", apiURL, projectID), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to get project info: %w", err)
			}
			defer resp.Body.Close()

			var project map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&project)

			for k, v := range project {
				fmt.Printf("%-15s %v\n", k+":", v)
			}
			return nil
		},
	}
}

func newProjectDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Printf("Delete project '%s'? This cannot be undone. [y/N] ", args[0])
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			req, err := http.NewRequest("DELETE",
				fmt.Sprintf("%s/api/v1/projects/%s", apiURL, args[0]), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to delete project: %w", err)
			}
			resp.Body.Close()

			fmt.Printf("Project '%s' deleted\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}
