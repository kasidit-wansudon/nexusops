package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newDeployCmd() *cobra.Command {
	var (
		environment string
		image       string
		strategy    string
		replicas    int
		wait        bool
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the current project",
		Long:  "Trigger a deployment for the current project using the specified strategy",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := getProjectID()
			if err != nil {
				return err
			}

			if image == "" {
				return fmt.Errorf("image is required (use --image)")
			}

			fmt.Printf("Deploying %s to %s...\n", image, environment)
			fmt.Printf("  Strategy: %s\n", strategy)
			fmt.Printf("  Replicas: %d\n", replicas)

			payload := map[string]interface{}{
				"environment": environment,
				"image":       image,
				"strategy":    strategy,
				"replicas":    replicas,
			}

			body, _ := json.Marshal(payload)
			url := fmt.Sprintf("%s/api/v1/projects/%s/deploy", apiURL, projectID)

			req, err := http.NewRequest("POST", url, bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("deployment request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
				var errResp map[string]string
				json.NewDecoder(resp.Body).Decode(&errResp)
				return fmt.Errorf("deployment failed: %s", errResp["error"])
			}

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			fmt.Printf("\nDeployment triggered successfully!\n")
			if deployID, ok := result["deployment_id"].(string); ok {
				fmt.Printf("  Deployment ID: %s\n", deployID)
			}

			if wait {
				return waitForDeployment(projectID, environment, timeout)
			}

			fmt.Println("\nUse 'nexusctl status' to check deployment progress")
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "production", "Target environment")
	cmd.Flags().StringVarP(&image, "image", "i", "", "Docker image to deploy")
	cmd.Flags().StringVarP(&strategy, "strategy", "s", "rolling", "Deployment strategy (rolling, blue-green, canary)")
	cmd.Flags().IntVarP(&replicas, "replicas", "r", 2, "Number of replicas")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for deployment to complete")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Deployment timeout")

	return cmd
}

func waitForDeployment(projectID, environment string, timeout time.Duration) error {
	fmt.Println("Waiting for deployment to complete...")

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("deployment timed out after %v", timeout)
			}

			status, err := getDeployStatus(projectID, environment)
			if err != nil {
				continue
			}

			switch status {
			case "running":
				fmt.Print(".")
			case "success", "completed":
				fmt.Printf("\n\nDeployment completed successfully!\n")
				return nil
			case "failed":
				return fmt.Errorf("\ndeployment failed")
			case "rolling_back":
				fmt.Printf("\nDeployment is rolling back...")
			}
		}
	}
}

func getDeployStatus(projectID, environment string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%s/status?env=%s", apiURL, projectID, environment)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result["status"], nil
}

func getProjectID() (string, error) {
	data, err := os.ReadFile("nexusops.yaml")
	if err != nil {
		return "", fmt.Errorf("nexusops.yaml not found — run 'nexusctl init' first")
	}

	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("name:")) {
			parts := bytes.SplitN(line, []byte(":"), 2)
			if len(parts) == 2 {
				return string(bytes.TrimSpace(parts[1])), nil
			}
		}
	}
	return "", fmt.Errorf("project name not found in nexusops.yaml")
}
