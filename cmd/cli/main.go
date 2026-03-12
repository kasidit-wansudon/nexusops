package main

import (
	"fmt"
	"os"

	"github.com/kasidit-wansudon/nexusops/cli/cmd"
)

const version = "1.0.0"

func main() {
	rootCmd := cmd.NewRootCmd(version)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
