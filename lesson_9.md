# Lesson 9: Production Deployment & Operations

## Learning Objectives
By the end of this lesson you will:
- ✅ Understand Temporal production architecture & persistence options
- ✅ Deploy Temporal and workers on Kubernetes (reference patterns)
- ✅ Scale workers horizontally & tune task queues
- ✅ Implement observability (metrics, tracing, structured logs)
- ✅ Plan workflow versioning & safe rollout strategies
- ✅ Secure Temporal (network, authn, authz, data protection)
- ✅ Establish backup & disaster recovery procedures
- ✅ Optimize cost & resource usage
- ✅ Define an operations runbook for this project

[← Back to Course Index](course.md) | [← Previous: Lesson 8](lesson_8.md)

---
## Why Before How: Reliability as a First-Class Requirement
Temporal is the backbone for long-lived business processes. Production concerns go beyond “it runs”: you must ensure uptime, resilience, observability, security, and evolvability without breaking in-flight executions.

---
## Temporal Server Production Architecture
```
Clients / Workers
   |       |
   v       v
┌─────────────────────────────────────────┐
│             Temporal Frontend           │  (gRPC: 7233)
└────────────────┬────────────────────────┘
                 │
        ┌────────┴─────────┐
        v                  v
  ┌────────────┐     ┌────────────┐
  │  History   │     │  Matching  │ (Task queue load balancing)
  └────┬───────┘     └────┬───────┘
       │                  │
       v                  v
  ┌────────────────────────────────────┐
  │          Persistence (DB)          │
  │  - Executions / History Events     │
  │  - Task Queues                     │
  │  - Visibility (search attributes)  │
  └────────────────────────────────────┘
```

### Persistence Options
| DB | Pros | Cons | Notes |
|----|------|------|-------|
| PostgreSQL | Familiar, strong consistency | Less horizontal scaling | Use Temporal's SQL schema migration tooling |
| MySQL | Similar to Postgres | Same limitations | Choose one; avoid multi vendor complexity |
| Cassandra | Scales horizontally | Operationally complex | Higher write throughput; temporal teams often recommend Postgres for simpler ops |
| SQLite (dev) | Simple local dev | Not for prod | Local testing only |

For this project: **PostgreSQL** is sufficient.

---
## Recommended Production Deployment (Kubernetes)
Components:
| Component | Deployment Kind | Scaling |
|-----------|-----------------|--------|
| Temporal Server | StatefulSet (DB separate) | Scale frontend / history / matching pods individually |
| PostgreSQL | Managed (Cloud SQL / RDS) or StatefulSet | Use managed service for simplicity |
| Workers | Deployment | Horizontal Pod Autoscaler based on queue backlog / CPU |
| Admin Tools | Job or ephemeral Pod | Run when needed |
| UI | Deployment | Scale 1–2 replicas |

### Sample Helm-Based Install
Use official Helm charts:
```bash
helm repo add temporal https://charts.temporal.io
helm install temporal-prod temporal/temporal \
  --set server.config.persistence.default.sql.user=temporal \
  --set server.config.persistence.default.sql.password=****** \
  --set server.config.persistence.default.sql.host=postgres.default.svc.cluster.local \
  --set server.config.persistence.default.sql.database=temporal \
  --set server.config.global.publicNamespace=default
```

### Worker Deployment Example (K8s YAML sketch)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-workers
spec:
  replicas: 3
  selector:
    matchLabels: {app: order-workers}
  template:
    metadata:
      labels: {app: order-workers}
    spec:
      containers:
      - name: worker
        image: your-registry/temporal-worker:latest
        command: ["/app/worker"]
        env:
        - name: TEMPORAL_HOST
          value: temporal-frontend.default.svc.cluster.local:7233
        - name: ORDER_TASK_QUEUE
          value: order-task-queue
        resources:
          limits: {cpu: "500m", memory: "512Mi"}
          requests: {cpu: "250m", memory: "256Mi"}
        livenessProbe:
          httpGet: {path: /healthz, port: 8081}
        readinessProbe:
          httpGet: {path: /ready, port: 8081}
```
Expose health endpoints in worker (optional simple http server) to allow rolling updates without killing in-flight tasks prematurely.

---
## Scaling Strategy
| Aspect | Strategy |
|--------|----------|
| Task Queue Backlog | Monitor with metrics; scale worker replicas when backlog > threshold |
| Activity Throughput | Split heavy activity classes into dedicated queues (`payment-task-queue`) |
| CPU/Memory | Right-size activity concurrency (`MaxConcurrentActivityExecutionSize`) |
| Horizontal Scaling | Workers are stateless → scale out easily |
| Vertical Scaling (Server) | Increase frontend/history/matching replicas independently |

### Dynamic Concurrency Tuning
Set worker options based on pod resources:
```go
worker.Options{
  MaxConcurrentActivityExecutionSize: 500,  // CPU-bound vs IO-bound? adjust
  MaxConcurrentWorkflowTaskExecutionSize: 200,
}
```
Monitor saturation: high queue latency + low worker CPU → increase concurrency; high CPU → scale replicas.

---
## Observability
### Metrics
Prometheus integration via SDK / interceptors:
- Activity latency histogram
- Workflow execution duration
- Retry attempt counters
- Signal wait duration

### Tracing
OpenTelemetry exporter:
```go
// Install OTEL SDK, wrap activity and workflow interceptors
// Export spans: workflow start, activity execute, retry attempt
```
Trace `OrderWorkflow` across payment service calls.

### Logging
Structure all logs with:
- WorkflowID, RunID
- TaskQueue
- Stage / Activity name
- Attempt number (from context)

Use log aggregation (ELK / Loki / Cloud Logging). Add correlation IDs for upstream HTTP requests triggering workflows.

### Visibility Attributes
Add search attributes for querying workflows (requires server config):
```go
workflow.GetLogger(ctx).Info("Tagging workflow")
// Set via StartWorkflowOptions: SearchAttributes: map[string]interface{}{"OrderID": orderID, "CustomerTier": tier}
```
Query workflows later by `OrderID` or `CustomerTier` (requires registered search attributes in Temporal cluster).

---
## Workflow Versioning & Rollouts
| Strategy | Description |
|----------|-------------|
| `GetVersion` Guards | Protect branching changes; old executions stay on old path |
| Canary Workers | Run new code on subset of workers by task queue sharding (`order-task-queue-v2`) |
| Time-Based Freeze | Delay code removal until all old runs complete |
| Dual Registration | Register old + new workflow names temporarily |

### Rollout Playbook
1. Add new logic behind `GetVersion`.
2. Deploy workers with new code (no downtime; old runs unaffected).
3. Monitor new executions for errors.
4. After all old runs complete, refactor default version.
5. Remove stale branching after several release cycles.

---
## Security
| Area | Concern | Control |
|------|---------|---------|
| Network | Open gRPC port | Restrict access with ingress/NLB + firewall rules |
| Data | Sensitive payloads in history | Encrypt application-level fields (or use encryption plugin) |
| AuthN | Any client can connect | mTLS between worker and server (Temporal supports TLS) |
| AuthZ | Unrestricted namespace usage | Use namespaces per domain (e.g., `orders`, `auth`) and apply namespace-level policies |
| Secrets | Payment API keys in activities | Use external secret manager (Vault, AWS Secrets Manager) injected via env/sidecar |
| Logs | PII leakage | Redact or hash sensitive data before logging |

Enable TLS:
```yaml
server:
  config:
    global:
      tls:
        frontend:
          server:
            certFile: /etc/temporal/tls/server.crt
            keyFile:  /etc/temporal/tls/server.key
          client:
            rootCaFiles: [/etc/temporal/tls/ca.crt]
```
Workers dial with TLS options.

---
## Backup & Disaster Recovery (DR)
| Area | Strategy |
|------|----------|
| DB Backups | Daily snapshots + PITR (Point-In-Time Recovery) |
| Schema Migrations | Version-controlled; tested in staging first |
| History Export | Optional: periodic export of critical workflow histories for audit |
| Cluster Failure | Use managed DB + multi AZ; redeploy stateless server quickly |
| Namespace Migration | Use Temporal namespace replication features (enterprise) |

**Recovery Steps:**
1. Restore DB snapshot.
2. Re-deploy Temporal server pointing to restored DB.
3. Workers reconnect; in-flight workflows resume.
4. Audit histories for partial external side-effects; trigger compensations manually if needed.

---
## Cost Optimization
| Cost Driver | Optimization |
|-------------|--------------|
| Over-provisioned Workers | Right-size concurrency; use HPA metrics |
| Chatty Activities | Batch small calls; reduce activity granularity if excessive |
| Long Polling | Proper timeouts; reuse clients |
| High Retry Storm | Tune retry backoff; classify non-retryable errors |
| DB Storage | Purge completed workflow histories after retention (namespace retention policy) |
| Unused Task Queues | Consolidate low-traffic queues |

---
## Operations Runbook (Template)
| Scenario | Action |
|----------|--------|
| High activity failure rate | Check error types; adjust retry or fix upstream service |
| Task queue backlog > threshold | Scale worker replicas; inspect slow activities |
| Workflow non-determinism detected | Halt new deploy; patch code; replay test; redeploy |
| DB latency spikes | Investigate slow queries; scale DB; add indexes |
| TLS handshake failures | Validate certificate expiry; rotate certs |
| Approval signals delayed | Inspect signal sending client logs; verify network reachability |

**Alert Threshold Examples:**
- `activity_retry_attempts{activity="ProcessPayment"} > 10` over 5m
- `workflow_duration_seconds{workflow="OrderWorkflow"} > 3600` (unexpected long tail)
- `task_queue_backlog > 1000`
- `worker_panic_count > 0`

---
## Production Readiness Checklist
| Item | Status |
|------|--------|
| Postgres HA (multi-AZ) | Pending |
| Automated DB Backups | Pending |
| TLS Enabled | Pending |
| Namespaces Defined (`orders`, `default`) | Pending |
| Retry Policies Tuned | Partial |
| Observability (metrics + logs) | Partial |
| Tracing | Pending |
| Workflow Version Guards | Implemented in OrderWorkflow |
| Secrets Externalized | Pending |
| Runbook & Alerts | Draft |
| CI Replay Tests | Planned |

Fill this out during staging rollout.

---
## Example: Environment Variables for Worker Pods
| Var | Purpose |
|-----|---------|
| TEMPORAL_HOST | Temporal server endpoint |
| ORDER_TASK_QUEUE | Primary workflow task queue |
| PAYMENT_TASK_QUEUE | Dedicated payment queue |
| LOG_LEVEL | Tuning verbosity |
| OTEL_EXPORTER_OTLP_ENDPOINT | Tracing exporter |
| ENABLE_TRACING | Feature flag |
| RETRY_MAX_ATTEMPTS | Override defaults via injection |

---
## Exercise
1. Draft Kubernetes manifests for: worker Deployment + service account with restricted permissions.
2. Add environment variables for payment task queue separation.
3. Implement basic metrics interceptor counting activity attempts (pseudo-code accepted).
4. Write a runbook entry for “Payment gateway outage” including manual compensation steps.
5. Define alert rule: if `OrderWorkflow` cancellations > X in 10m → page ops.
6. Create a staging rollout plan: start with 1 worker replica, gradually increase to 3 while monitoring metrics.
7. Add a `SearchAttribute` for `CustomerTier`, query workflows by tier in UI.

---
## Troubleshooting Table
| Symptom | Cause | Mitigation |
|---------|-------|------------|
| Persistent high backlog | Under-scaled workers | Increase replicas or concurrency settings |
| Random workflow failures post deploy | Non-deterministic change | Roll back; add replay test in CI |
| Slow payment activities | External API latency | Increase timeout; add circuit breaker outside workflow |
| DB connection saturation | Too many worker clients | Reuse single client per process |
| Large workflow histories | Excessive event granularity | Combine steps; avoid micro-activities |
| Signal loss (rare) | Client error / wrong ID | Validate IDs; add sender retries |
| TLS errors | Expired cert | Automate rotation (cert-manager) |

---
## What You've Learned
✅ Temporal production architecture & persistence choices  
✅ Kubernetes deployment patterns & scaling  
✅ Observability pillars (metrics, logs, tracing)  
✅ Security, TLS, secrets management  
✅ Versioning & safe rollout strategies  
✅ DR & backup planning  
✅ Cost optimization levers  
✅ Operational runbook & readiness checklist  

---
## What's Next?

You've completed the core Temporal course covering fundamentals through production deployment!

**Optional Advanced Topic:**
If you want to dive deeper into compensation patterns and the Saga pattern, continue to:
👉 **[Lesson 10: Compensation & Saga Patterns Deep Dive](lesson_10.md)**

This advanced lesson covers:
- Saga pattern for distributed transactions
- Complex compensation scenarios
- Real-world examples (travel booking, bank transfers)
- Testing and monitoring sagas

**Or, start building:**
- Implement missing production items (TLS, metrics, runbook)
- Add real domain activities (payment API, stock DB)
- Automate CI replay tests
- Socialize runbook with team

If you need a compressed summary, ask: "Save progress to temporal/compressed.md".

[← Back to Course Index](course.md) | [← Previous: Lesson 8](lesson_8.md) | [Next: Lesson 10 (Advanced) →](lesson_10.md)

