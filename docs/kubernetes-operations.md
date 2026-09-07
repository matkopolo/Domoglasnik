# Kubernetes operations

Use an explicit production kubeconfig/context. Resource names below match the
manifests. See [bootstrap and upgrades](k3s-deployment.md) and
[backup/recovery](backup-and-recovery.md). No GitHub CLI or cluster-admin CI token
is needed.

## Inspect and diagnose

```sh
kubectl get nodes -o wide
kubectl get pods -A
kubectl -n real-estate get all
kubectl -n real-estate get cluster,backup,scheduledbackup,objectstore,pvc
kubectl -n real-estate logs deployment/real-estate-api -f
kubectl -n real-estate logs deployment/web -f
kubectl -n real-estate logs deployment/pipeline-worker -f
kubectl -n real-estate describe pod POD_NAME
kubectl -n real-estate logs POD_NAME --previous
kubectl get events -A --sort-by=.lastTimestamp
kubectl top nodes
kubectl top pods -n real-estate --containers
kubectl -n real-estate rollout status deployment/real-estate-api
kubectl -n real-estate rollout history deployment/real-estate-api
kubectl -n real-estate rollout undo deployment/real-estate-api
kubectl -n real-estate rollout status deployment/web
kubectl -n real-estate exec deployment/real-estate-api -- id
kubectl -n real-estate port-forward service/real-estate-api 8081:8080
# In another terminal:
curl -fsS http://127.0.0.1:8081/health/live
curl -fsS http://127.0.0.1:8081/health/ready
```

Logs go to stdout/stderr; use `--since=1h` / `--timestamps` to bound inspection.
Do not dump environment variables or Secret data while debugging. Keep
port-forward bound to localhost, not 0.0.0.0. A Deployment rollback changes app
code, **not migrated schema or analytical publications**. Roll back only to a
schema-compatible release. Review rollout image digests, not just tag names.

API liveness checks process health independently of PostgreSQL. DB outage changes
readiness to 503; do not restart a healthy API just to mask a DB outage. Web probes
check HTTP / independently of backend availability. Worker readiness indicates a
completed cycle in that process; absence/freshness of data must also be checked
using publication status and logs. There is deliberately no time-based worker
liveness test that kills healthy imports/model work.

## Database operations (no pod address in application configuration)

```sh
kubectl -n real-estate get cluster real-estate-db -o wide
kubectl -n real-estate get cluster real-estate-db -o jsonpath='{.status.currentPrimary}{"\n"}'
kubectl -n real-estate get pods -l cnpg.io/cluster=real-estate-db -o wide
kubectl -n real-estate get endpointslice -l kubernetes.io/service-name=real-estate-db-rw
PRIMARY=$(kubectl -n real-estate get cluster real-estate-db -o jsonpath='{.status.currentPrimary}')
kubectl -n real-estate logs "$PRIMARY" -c postgres --since=1h
kubectl -n real-estate logs "$PRIMARY" -c plugin-barman-cloud --since=1h
kubectl -n real-estate exec "$PRIMARY" -c postgres -- psql -U postgres -d real_estate -c \
  "SELECT application_name,state,sync_state,replay_lag FROM pg_stat_replication"
kubectl -n real-estate exec "$PRIMARY" -c postgres -- psql -U postgres -d real_estate -c \
  "SELECT filename,sha256,applied_at FROM public.production_schema_migrations ORDER BY filename"
kubectl -n real-estate exec "$PRIMARY" -c postgres -- psql -U postgres -d real_estate -c \
  "SELECT * FROM analytics.analytical_publication"
kubectl -n real-estate get backup,scheduledbackup,objectstore
```

If the sidecar name changes on a deliberate plugin upgrade, inspect pod
`.spec.containers[*].name`/`.spec.initContainers[*].name` first. CNPG runs the
plugin as a native sidecar. Optional convenience: install the matching
`kubectl cnpg` plugin and run `kubectl cnpg status -n real-estate real-estate-db`.
All required operations above work through ordinary kubectl.

Inspect replica recovery status individually:
`SELECT pg_is_in_recovery(),pg_last_wal_receive_lsn(),pg_last_wal_replay_lsn(),pg_last_xact_replay_timestamp();`.
Use the primary's current WAL LSN to check catch-up; replay timestamp can be old
on an idle database and is not alone evidence of lag.

## Explicit maintenance Jobs

Set the release/context variables from the deployment runbook. During schema
migrations or long manual work, pause the scheduler at a safe point: inspect logs,
wait for the active cycle to finish, then scale to zero. Do not casually interrupt
imports. The advisory lock is the final exclusion guard if operations overlap.

```sh
kubectl -n real-estate scale deployment/pipeline-worker --replicas=0
kubectl -n real-estate wait --for=delete pod -l app.kubernetes.io/name=pipeline-worker --timeout=900s
JOB=$(sh infrastructure/kubernetes/scripts/operator.sh job single-node analytics-refresh)
# Or deliberately: ... job single-node models-refresh
kubectl -n real-estate get "$JOB"
kubectl -n real-estate describe "$JOB"
kubectl -n real-estate logs -f "$JOB"
kubectl -n real-estate wait --for=condition=Complete "$JOB" --timeout=24h
kubectl -n real-estate delete "$JOB"
kubectl -n real-estate scale deployment/pipeline-worker --replicas=1
```

The delete above removes only the specific completed Job and its logs/pods, not
database changes. Save required logs first; a 7-day Job TTL also cleans completed
objects. If the Job fails, do not blindly train again: inspect the cause. No full
historical import/model training runs in CI. There is no destructive reset helper
for production. Reset/repopulate is a separately approved data-loss operation.

The scheduler remains exactly one active process. Both overlays use Recreate to
avoid rollout overlap; PostgreSQL locks also cover manual analytics/model commands
and migration Jobs. HA does not mean scheduling multiple workers.

## Capacity and security

Initial per-container requests/limits:

| Workload | Single / HA count | CPU request / limit | Memory request / limit |
| --- | --- | --- | --- |
| API | 1 / 2 | 100m / 1 | 128Mi / 512Mi |
| Web | 1 / 2 | 100m / 1 | 256Mi / 768Mi |
| Worker | 1 / 1 | 500m / 2 | 1Gi / 4Gi |
| CNPG database | 1 / 3 | 500m / 2 | 2Gi / 4Gi each |
| Barman sidecar | 1 / 3 | 100m / 1 | 128Mi / 512Mi each |
| K3s Traefik | 1 / 2 | 100m / 1 | 128Mi / 512Mi each |
| Manual analytics/model/bootstrap Job | only on demand | 250m / 2 | 1Gi / 4Gi |
| Migration Job | only on demand | 250m / 2 | 128Mi / 512Mi |

Add OS/K3s/Traefik/cert-manager/CNPG-controller/plugin overhead and surge pods.
An HA node must have capacity for failover workloads, not just normal averages.
Patch sizing in your environment overlay after measuring CPU, RSS, database
connections, disk growth/IOPS and model peaks. These are starting budgets, not
load-tested capacity guarantees. Worker/model parallelism is explicitly 2, native
math-library threads 1. API pools use at most 10 connections each; PostgreSQL
max_connections100 leaves headroom for jobs, replication and operators.

App containers run numeric nonroot UIDs, no privilege escalation, dropped
capabilities, RuntimeDefault seccomp, read-only roots, bounded scratch/cache
emptyDirs and no mounted Kubernetes API token. CNPG controls its own pod security.
No app needs privileged/hostNetwork/hostPID. The privileged container in the
disposable **test-cluster script** is K3s-in-Docker, not a production workload.

Ingress policies admit only Traefik to web/API/ACME solvers and internal DB clients,
replication and CNPG operator traffic to PostgreSQL. Egress is deliberately not
blanket-blocked: GURS download redirects, public geographic providers, DNS and S3
need outbound connectivity. Measure/allowlist before adding egress policy. Network
policies do not replace TLS, DB grants, node firewall or kubeconfig/RBAC security.

## Three-node failure acceptance drills

Run only in an agreed maintenance window with verified backups and rollback
ownership. Do NOT run these on single-node production.

### Application node drain

1. Confirm three Ready nodes, both web/API replicas Ready and on distinct nodes.
2. Record baseline HTTPS requests continuously (not just one successful curl).
3. Select a node; `kubectl drain NODE --ignore-daemonsets --delete-emptydir-data`.
   This discards ephemeral scratch/cache on that node; do not add force or bypass
   PDBs to “make it work.” If CNPG/PVC/PDB restrictions block the drain, stop and
   inspect. Expect temporary loss of a node-local worker if its node is drained.
4. Verify surviving web/API availability and replacement pod readiness/spread.
5. `kubectl uncordon NODE`; verify the full healthy topology returns.

### PostgreSQL primary-node failure/failover

1. Verify one primary, two streaming/caught-up replicas on distinct physical
   nodes, healthy etcd quorum, off-node backup and successful WAL archiving.
2. Identify current primary and its node using commands above.
3. Start a continuous read/write application probe with identifiable test data
   and record times/results. Use a dedicated non-sensitive test account.
4. Controlled-drain the primary node, or perform an explicitly approved node
   outage simulation if testing abrupt failure. Never fail two etcd servers.
5. Observe CNPG—not a custom script—promote a replica. Inspect Cluster events,
   .status.currentPrimary, replication state and real-estate-db-rw EndpointSlice.
6. Verify application connections recover through the unchanged RW Service,
   readiness returns and queries/data are consistent. There may be connection
   failures during detection/promotion; zero interruption is not promised.
7. Restore/uncordon the original node. Verify primary+two healthy replicas,
   etcd quorum, archived WAL, backup freshness and all app pods.

The default database replication is asynchronous. HA availability does not
promise zero acknowledged-write loss under abrupt primary/storage failure.
If the required RPO is zero, assess CNPG-supported synchronous replication and
its write-availability trade-off before changing the HA overlay.

No Redis drill is applicable today. The future Sentinel acceptance procedure and
asynchronous durability limits are documented in infrastructure/kubernetes/cache.
