package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	apiURL  string
	apiKey  string
	verbose bool
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "nexusctl",
		Short: "NexusOps CLI — manage your developer platform",
		Long: `nexusctl is the command-line interface for NexusOps,
a self-hosted developer platform combining CI/CD, deployment,
monitoring, and team collaboration.`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				fmt.Printf("API URL: %s\n", apiURL)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "NexusOps API server URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(
		newInitCmd(),
		newDeployCmd(),
		newLogsCmd(),
		newEnvCmd(),
		newStatusCmd(),
		newRollbackCmd(),
		newProjectCmd(),
		newPipelineCmd(),
	)

	return rootCmd
}
