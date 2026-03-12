package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfig = `# NexusOps Configuration
# Documentation: https://nexusops.dev/docs/config

name: %s
version: "1.0"

build:
  dockerfile: Dockerfile
  context: .
  cache:
    - node_modules
    - .go/pkg

pipeline:
  stages:
    - name: test
      steps:
        - name: lint
          image: golangci/golangci-lint:latest
          commands:
            - golangci-lint run ./...
        - name: unit-tests
          image: golang:1.22
          commands:
            - go test ./...
          cache:
            key: go-mod-${CHECKSUM:go.sum}
            paths:
              - /go/pkg/mod

    - name: build
      steps:
        - name: build-binary
          image: golang:1.22
          commands:
            - CGO_ENABLED=0 go build -o /app/server ./cmd/api
          artifacts:
            paths:
              - /app/server
          depends_on:
            - test

    - name: deploy
      steps:
        - name: deploy-production
          image: nexusops/deployer:latest
          commands:
            - nexusctl deploy --image ${IMAGE} --env production
          depends_on:
            - build

deploy:
  provider: docker
  strategy: rolling
  replicas: 2
  port: 8080
  health_check:
    path: /health
    interval: 30s
    timeout: 10s
    retries: 3
  resources:
    cpu: "500m"
    memory: "256Mi"

notifications:
  slack:
    webhook_url: "${SLACK_WEBHOOK_URL}"
    channel: "#deployments"
`

func newInitCmd() *cobra.Command {
	var (
		name      string
		force     bool
		template  string
	)

	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Initialize a new NexusOps project",
		Long:  "Creates a nexusops.yaml configuration file in the current directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				name = args[0]
			}
			if name == "" {
				dir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				name = filepath.Base(dir)
			}

			configPath := "nexusops.yaml"
			if _, err := os.Stat(configPath); err == nil && !force {
				return fmt.Errorf("nexusops.yaml already exists (use --force to overwrite)")
			}

			var content string
			switch template {
			case "go":
				content = fmt.Sprintf(defaultConfig, name)
			case "node":
				content = generateNodeConfig(name)
			case "laravel":
				content = generateLaravelConfig(name)
			default:
				content = fmt.Sprintf(defaultConfig, name)
			}

			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Printf("Initialized NexusOps project '%s'\n", name)
			fmt.Println("Created nexusops.yaml")
			fmt.Println("\nNext steps:")
			fmt.Println("  1. Edit nexusops.yaml to match your project")
			fmt.Println("  2. Run 'nexusctl deploy' to deploy")
			fmt.Println("  3. Run 'nexusctl status' to check deployment status")
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Project name")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing config")
	cmd.Flags().StringVarP(&template, "template", "t", "go", "Config template (go, node, laravel)")

	return cmd
}

func generateNodeConfig(name string) string {
	return fmt.Sprintf(`name: %s
version: "1.0"

build:
  dockerfile: Dockerfile
  context: .
  cache:
    - node_modules
    - .next/cache

pipeline:
  stages:
    - name: test
      steps:
        - name: lint
          image: node:20-alpine
          commands:
            - npm ci
            - npm run lint
        - name: unit-tests
          image: node:20-alpine
          commands:
            - npm ci
            - npm test
          cache:
            key: npm-${CHECKSUM:package-lock.json}
            paths:
              - node_modules

    - name: build
      steps:
        - name: build-app
          image: node:20-alpine
          commands:
            - npm ci
            - npm run build
          artifacts:
            paths:
              - .next/
              - dist/
          depends_on:
            - test

deploy:
  provider: docker
  strategy: rolling
  replicas: 2
  port: 3000
  health_check:
    path: /api/health
    interval: 30s
    timeout: 10s
    retries: 3
`, name)
}

func generateLaravelConfig(name string) string {
	return fmt.Sprintf(`name: %s
version: "1.0"

build:
  dockerfile: Dockerfile
  context: .
  cache:
    - vendor
    - node_modules

pipeline:
  stages:
    - name: test
      steps:
        - name: php-tests
          image: php:8.3-cli
          commands:
            - composer install --no-interaction
            - php artisan test
          services:
            - name: mysql
              image: mysql:8.0
              env:
                MYSQL_DATABASE: testing
                MYSQL_ROOT_PASSWORD: secret

    - name: build
      steps:
        - name: build-assets
          image: node:20-alpine
          commands:
            - npm ci
            - npm run build
          depends_on:
            - test

deploy:
  provider: docker
  strategy: blue-green
  replicas: 2
  port: 8000
  health_check:
    path: /up
    interval: 30s
    timeout: 10s
    retries: 3
`, name)
}
