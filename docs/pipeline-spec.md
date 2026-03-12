# Pipeline Specification

NexusOps pipelines are defined in `nexusops.yaml` using a declarative YAML format.

## Top-Level Fields

```yaml
name: string          # Required. Project name
version: string       # Required. Config version

build:                # Build configuration
  dockerfile: string  # Path to Dockerfile (default: Dockerfile)
  context: string     # Build context (default: .)
  cache: []string     # Directories to cache between builds

pipeline:             # Pipeline definition
  stages: []Stage     # Ordered list of pipeline stages

deploy:               # Deployment configuration
  provider: string    # "docker" or "kubernetes"
  strategy: string    # "rolling", "blue-green", or "canary"
  replicas: int       # Number of replicas
  port: int           # Service port
  health_check:       # Health check configuration
  resources:          # Resource limits

notifications:        # Notification channels
  slack:
  discord:
  email:
```

## Stage Definition

```yaml
stages:
  - name: string          # Required. Unique stage name
    steps: []Step          # Required. Steps in this stage
```

## Step Definition

```yaml
steps:
  - name: string          # Required. Unique step name
    image: string         # Required. Docker image for execution
    commands: []string    # Required. Commands to execute
    env: map[string]string  # Environment variables
    cache:                # Step-level caching
      key: string         # Cache key (supports ${CHECKSUM:file} template)
      paths: []string     # Paths to cache
    artifacts:            # Build artifacts
      paths: []string     # Paths to collect as artifacts
    services: []Service   # Sidecar services
    depends_on: []string  # Stage dependencies (DAG edges)
    timeout: string       # Step timeout (e.g., "30m")
    retry: int            # Retry count on failure
```

## Service Definition (Sidecars)

```yaml
services:
  - name: string          # Service name
    image: string         # Docker image
    env: map[string]string  # Environment variables
    ports: []string       # Port mappings
```

## Cache Key Templates

Cache keys support variable substitution:

- `${CHECKSUM:file}` — SHA256 checksum of a file (e.g., `go.sum`, `package-lock.json`)
- `${BRANCH}` — Current git branch
- `${COMMIT}` — Current git commit SHA

Example:
```yaml
cache:
  key: go-mod-${CHECKSUM:go.sum}
  paths:
    - /go/pkg/mod
```

## Dependency Resolution

Pipeline stages support explicit dependencies via `depends_on`. NexusOps performs:

1. **Topological sorting** — Stages execute in dependency order
2. **Parallel execution** — Independent stages run concurrently
3. **Cycle detection** — Circular dependencies are rejected at parse time

Example DAG:
```
test ──→ build ──→ deploy
  │                  ↑
  └──→ security ─────┘
```

```yaml
stages:
  - name: test
    steps: [...]

  - name: security
    steps: [...]
    depends_on: [test]

  - name: build
    steps: [...]
    depends_on: [test]

  - name: deploy
    steps: [...]
    depends_on: [build, security]
```

## Deploy Configuration

### Docker Provider

```yaml
deploy:
  provider: docker
  strategy: rolling    # rolling | blue-green | canary
  replicas: 3
  port: 8080
  health_check:
    path: /health
    interval: 30s
    timeout: 10s
    retries: 3
  resources:
    cpu: "500m"
    memory: "256Mi"
  env:
    NODE_ENV: production
```

### Kubernetes Provider

```yaml
deploy:
  provider: kubernetes
  namespace: production
  strategy: rolling
  replicas: 3
  port: 8080
  health_check:
    path: /health
    interval: 30s
    timeout: 10s
    retries: 3
  resources:
    cpu: "500m"
    memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "512Mi"
  ingress:
    host: api.example.com
    tls: true
```

## Notifications

```yaml
notifications:
  slack:
    webhook_url: "${SLACK_WEBHOOK_URL}"
    channel: "#deployments"
    events: [deploy.success, deploy.failure, pipeline.failure]

  discord:
    webhook_url: "${DISCORD_WEBHOOK_URL}"
    events: [deploy.success, deploy.failure]

  email:
    recipients:
      - team@example.com
    events: [deploy.failure]
```

## Complete Example

```yaml
name: my-api
version: "1.0"

build:
  dockerfile: Dockerfile
  context: .
  cache:
    - .go/pkg

pipeline:
  stages:
    - name: test
      steps:
        - name: lint
          image: golangci/golangci-lint:latest
          commands:
            - golangci-lint run ./...
          timeout: "5m"

        - name: unit-tests
          image: golang:1.22
          commands:
            - go test -race -coverprofile=coverage.out ./...
          cache:
            key: go-mod-${CHECKSUM:go.sum}
            paths:
              - /go/pkg/mod
          artifacts:
            paths:
              - coverage.out

        - name: integration-tests
          image: golang:1.22
          commands:
            - go test -tags=integration ./...
          services:
            - name: postgres
              image: postgres:16-alpine
              env:
                POSTGRES_DB: test
                POSTGRES_PASSWORD: test
          retry: 2

    - name: build
      steps:
        - name: build-binary
          image: golang:1.22
          commands:
            - CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/api
          artifacts:
            paths:
              - /app/server
      depends_on: [test]

    - name: deploy
      steps:
        - name: deploy-staging
          image: nexusops/deployer:latest
          commands:
            - nexusctl deploy --env staging --strategy rolling
      depends_on: [build]

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
```
