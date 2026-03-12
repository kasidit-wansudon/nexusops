# Deployment Strategies

NexusOps supports three deployment strategies, each suited to different reliability and availability requirements.

## Rolling Update

The default strategy. Gradually replaces old instances with new ones.

```yaml
deploy:
  strategy: rolling
  replicas: 4
```

### How it works

1. New container is started alongside existing containers
2. Health check verifies the new container is healthy
3. Traffic is shifted to include the new container
4. One old container is removed
5. Repeat until all containers are replaced

### Characteristics

- **Zero downtime** — old containers serve traffic until new ones are ready
- **Gradual rollout** — issues are caught early before full deployment
- **Resource overhead** — temporarily runs N+1 containers during deployment
- **Rollback** — automatic rollback if health checks fail

### Best for

- Stateless services
- APIs and web servers
- Services where brief mixed-version traffic is acceptable

## Blue-Green

Maintains two identical environments, switching traffic atomically.

```yaml
deploy:
  strategy: blue-green
  replicas: 2
```

### How it works

1. **Blue** environment is currently serving traffic
2. **Green** environment is deployed with the new version
3. Health checks verify the green environment
4. Proxy switches all traffic from blue to green atomically
5. Blue environment is kept as rollback target
6. Blue environment is torn down after confirmation period

### Characteristics

- **Zero downtime** — atomic traffic switch
- **Instant rollback** — switch back to blue environment
- **Double resources** — both environments run simultaneously during deployment
- **No mixed versions** — all traffic goes to one version

### Best for

- Critical production services
- Database migrations that need instant rollback
- Services where version consistency is important

## Canary

Routes a small percentage of traffic to the new version before full rollout.

```yaml
deploy:
  strategy: canary
  replicas: 4
```

### How it works

1. Deploy one canary instance with the new version
2. Route 10% of traffic to the canary
3. Monitor error rates and latency for the canary
4. If healthy, gradually increase traffic (25%, 50%, 75%, 100%)
5. If unhealthy, automatically route all traffic back to stable version

### Characteristics

- **Risk mitigation** — issues affect only a fraction of traffic
- **Data-driven** — promotion based on real traffic metrics
- **Slower rollout** — full deployment takes longer
- **Automatic rollback** — reverts on anomaly detection

### Best for

- High-traffic services where full deployment risk is unacceptable
- A/B testing new features
- Services with strict SLA requirements

## Health Checks

All strategies rely on health checks to determine deployment success:

```yaml
deploy:
  health_check:
    path: /health        # HTTP endpoint to check
    interval: 30s        # Time between checks
    timeout: 10s         # Maximum response time
    retries: 3           # Failures before marking unhealthy
```

### Health Check Types

| Type | Description | Configuration |
|------|-------------|---------------|
| HTTP | GET request to endpoint, expects 2xx | `path`, `interval`, `timeout` |
| TCP | TCP connection attempt | `port`, `interval`, `timeout` |
| Command | Execute command in container | `command`, `interval`, `timeout` |

### Automatic Rollback

When health checks fail during deployment:

1. Deployment is marked as failed
2. New containers are stopped
3. Previous version is restored
4. Notification is sent to configured channels
5. Event is logged in the activity feed

## Choosing a Strategy

| Requirement | Rolling | Blue-Green | Canary |
|-------------|---------|------------|--------|
| Zero downtime | Yes | Yes | Yes |
| Instant rollback | Partial | Yes | Yes |
| Resource efficiency | High | Low | Medium |
| Risk mitigation | Medium | High | Highest |
| Deployment speed | Fast | Medium | Slow |
| Version consistency | No | Yes | No |

## CLI Usage

```bash
# Deploy with specific strategy
nexusctl deploy --strategy rolling
nexusctl deploy --strategy blue-green
nexusctl deploy --strategy canary

# Monitor deployment
nexusctl status --watch

# Manual rollback
nexusctl rollback --version v1.2.3
```
