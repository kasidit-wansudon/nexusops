package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables",
		Long:  "Set, get, list, and delete encrypted environment variables",
	}

	cmd.AddCommand(
		newEnvSetCmd(),
		newEnvGetCmd(),
		newEnvListCmd(),
		newEnvDeleteCmd(),
		newEnvImportCmd(),
		newEnvExportCmd(),
	)

	return cmd
}

func newEnvSetCmd() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "set KEY=VALUE [KEY=VALUE...]",
		Short: "Set environment variables",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			for _, arg := range args {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid format: %s (expected KEY=VALUE)", arg)
				}

				key, value := parts[0], parts[1]
				payload, _ := json.Marshal(map[string]string{
					"key":         key,
					"value":       value,
					"environment": environment,
				})

				url := fmt.Sprintf("%s/api/v1/projects/%s/env", apiURL, projectID)
				req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+apiKey)

				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					return fmt.Errorf("failed to set %s: %w", key, err)
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("failed to set %s: server returned %d", key, resp.StatusCode)
				}

				fmt.Printf("Set %s for %s environment\n", key, environment)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	return cmd
}

func newEnvGetCmd() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Get an environment variable value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			url := fmt.Sprintf("%s/api/v1/projects/%s/env/%s?environment=%s",
				apiURL, projectID, args[0], environment)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to get variable: %w", err)
			}
			defer resp.Body.Close()

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			fmt.Println(result["value"])
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	return cmd
}

func newEnvListCmd() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			url := fmt.Sprintf("%s/api/v1/projects/%s/env?environment=%s",
				apiURL, projectID, environment)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to list variables: %w", err)
			}
			defer resp.Body.Close()

			type EnvVar struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				Environment string `json:"environment"`
				Encrypted   bool   `json:"encrypted"`
			}

			var vars []EnvVar
			json.NewDecoder(resp.Body).Decode(&vars)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "KEY\tVALUE\tENVIRONMENT\tENCRYPTED\n")
			fmt.Fprintf(w, "---\t-----\t-----------\t---------\n")
			for _, v := range vars {
				encrypted := "no"
				if v.Encrypted {
					encrypted = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.Key, v.Value, v.Environment, encrypted)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	return cmd
}

func newEnvDeleteCmd() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "delete KEY",
		Short: "Delete an environment variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			url := fmt.Sprintf("%s/api/v1/projects/%s/env/%s?environment=%s",
				apiURL, projectID, args[0], environment)

			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to delete variable: %w", err)
			}
			resp.Body.Close()

			fmt.Printf("Deleted %s from %s environment\n", args[0], environment)
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	return cmd
}

func newEnvImportCmd() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "import FILE",
		Short: "Import environment variables from a .env file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			vars := make(map[string]string)
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					vars[key] = value
				}
			}

			payload, _ := json.Marshal(map[string]interface{}{
				"variables":   vars,
				"environment": environment,
			})

			url := fmt.Sprintf("%s/api/v1/projects/%s/env/import", apiURL, projectID)
			req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}
			resp.Body.Close()

			fmt.Printf("Imported %d variables to %s environment\n", len(vars), environment)
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	return cmd
}

func newEnvExportCmd() *cobra.Command {
	var (
		environment string
		output      string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export environment variables to a .env file",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			url := fmt.Sprintf("%s/api/v1/projects/%s/env/export?environment=%s",
				apiURL, projectID, environment)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("export failed: %w", err)
			}
			defer resp.Body.Close()

			var vars map[string]string
			json.NewDecoder(resp.Body).Decode(&vars)

			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("# NexusOps Environment Export (%s)\n", environment))
			buf.WriteString(fmt.Sprintf("# Generated: %s\n\n", time.Now().Format(time.RFC3339)))
			for k, v := range vars {
				buf.WriteString(fmt.Sprintf("%s=%q\n", k, v))
			}

			if output != "" {
				if err := os.WriteFile(output, []byte(buf.String()), 0600); err != nil {
					return fmt.Errorf("failed to write file: %w", err)
				}
				fmt.Printf("Exported %d variables to %s\n", len(vars), output)
			} else {
				fmt.Print(buf.String())
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path")
	return cmd
}
