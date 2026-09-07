# K3s production deployment

This is an explicit operator runbook, not a one-command production installer.
Local development is still `docker compose up -d --build --wait`.
Read [readiness/limitations](kubernetes-readiness-report.md) first.

## 1. Physical node and cluster prerequisites

Use independent Linux machines with stable private IPs, reliable SSDs, time
synchronization, monitored free space and an off-node object store. Budget at least
8 CPU/16 GiB RAM for the first node as an initial sizing assumption, then measure;
model training and PostgreSQL dominate. Reserve OS/K3s/addon headroom beyond pod
limits. Do not expose 6443, etcd, PostgreSQL or kubelet to the Internet. Allow only
80/443 publicly; limit cluster ports to trusted nodes/operator access as documented
in [K3s requirements](https://docs.k3s.io/installation/requirements).
DNS/firewall/VPN/provisioning automation are intentionally outside this change.

Start **embedded etcd on node 1**, not the default SQLite datastore:

```sh
# Inspect the downloaded installer before executing it with root privileges.
curl -sfL https://get.k3s.io -o /tmp/install-k3s.sh
sudo env INSTALL_K3S_VERSION='v1.35.8+k3s1' sh /tmp/install-k3s.sh server \
  --cluster-init --secrets-encryption --write-kubeconfig-mode=600
sudo k3s kubectl get nodes
sudo k3s secrets-encrypt status
```

Set a stable private registration endpoint/TLS SAN in K3s configuration before
joining further nodes. Protect `/etc/rancher/k3s/k3s.yaml` (cluster-admin). Use a
separate scoped operator identity where possible. Copy kubeconfig through a secure
channel, change its server address to the private endpoint, and select it
explicitly. Keep the server token in a separate off-node secret vault: restoring
etcd with encrypted bootstrap data requires that token.

Retain bundled Traefik, ServiceLB (or deliberately replace ingress exposure later),
local-path and metrics-server. Do not expose management dashboards. Configure
[etcd S3 snapshots](backup-and-recovery.md#k3s-embedded-etcd-is-a-separate-backup)
before storing application credentials.

```sh
export KUBE_CONTEXT=your-production-context
kubectl --context "$KUBE_CONTEXT" get nodes
# Use this explicit context for every following bare kubectl command.
kubectl config use-context "$KUBE_CONTEXT"
kubectl apply -k infrastructure/kubernetes/addons/traefik/base
kubectl -n kube-system rollout status deployment/traefik --timeout=300s
. infrastructure/kubernetes/addons/versions.env
kubectl apply -f "$CERT_MANAGER_MANIFEST"
kubectl -n cert-manager rollout status deployment/cert-manager --timeout=300s
kubectl -n cert-manager rollout status deployment/cert-manager-webhook --timeout=300s
kubectl -n cert-manager rollout status deployment/cert-manager-cainjector --timeout=300s
kubectl apply --server-side -f "$CNPG_MANIFEST"
kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager --timeout=300s
kubectl apply --server-side -f "$BARMAN_MANIFEST"
kubectl -n cnpg-system rollout status deployment/barman-cloud --timeout=300s
kubectl wait --for=condition=Established crd/clusters.postgresql.cnpg.io --timeout=120s
kubectl wait --for=condition=Established crd/objectstores.barmancloud.cnpg.io --timeout=120s
kubectl apply -k infrastructure/kubernetes/security
```

Operator versions are pinned in `infrastructure/kubernetes/addons/versions.env`.
Review upstream manifests/security advisories before installation; this runbook
does not grant GitHub Actions any Kubernetes credentials.

## 2. Secrets and release configuration

GHCR publishing occurs after existing CI gates plus Kubernetes/migration checks,
on pushes to main. Images are web, api, pipeline and a small dedicated migrations
image, tagged `sha-<full Git SHA>` (amd64/arm64). Do not redeploy mutable latest.
GHCR tags are a convention, not registry-enforced immutability: protect publishing
permissions and record resolved image digests with each release. For strongest
immutability pin the release overlays to those recorded digests.

```sh
export IMAGE_TAG=sha-REPLACE_WITH_FULL_40_CHARACTER_RELEASE_SHA
export PUBLIC_HOST=your-real-hostname
export ACME_EMAIL=your-certificate-contact
export BACKUP_BUCKET=your-private-off-cluster-bucket
export BACKUP_ENDPOINT=https://your-s3-endpoint
```

Create Secret inputs outside the repository with mode 0600 and no trailing newline.
Do not place passwords in shell arguments, history, CI logs or committed YAML.
Use `kubectl create secret generic NAME --from-file=KEY=/secure/path ...`.
Required Secrets in namespace real-estate:

| Secret | Keys | Consumer |
| --- | --- | --- |
| real-estate-migration-db | username=postgres, password, uri | CNPG superuser; migration Job only |
| real-estate-api-db | username=real_estate_api, password, uri | CNPG managed role; API |
| real-estate-pipeline-db | username=real_estate_pipeline, password, uri | CNPG managed role; worker/jobs |
| backup-credentials | ACCESS_KEY_ID, ACCESS_SECRET_KEY | Barman sidecar |
| ghcr-pull (only if private images) | docker config JSON | workload image pulls |

Database Secrets should use type `kubernetes.io/basic-auth` (username/password;
extra `uri` key is allowed). The URI must use percent-encoded username/password:

```text
postgresql://USERNAME:URL_ENCODED_PASSWORD@real-estate-db-rw.real-estate.svc.cluster.local:5432/real_estate?sslmode=verify-full&sslrootcert=/etc/database-ca/ca.crt
```

The manifest mounts CNPG's generated `real-estate-db-ca` Secret in clients.
Never use pod IPs or individual primary pod names. Secret-backed URIs are required
because the existing application contract is DATABASE_URL. Keep password and URI
in sync on rotation; wait for CNPG managed-role reconciliation before rolling
clients. The DB owner is CNPG's bootstrap `real_estate` role; applications do not
mount its generated credentials. Historical migrations refer to that role.

Example using pre-created secure files:

```sh
kubectl -n real-estate create secret generic real-estate-api-db \
  --type=kubernetes.io/basic-auth \
  --from-file=username=/secure/api-username \
  --from-file=password=/secure/api-password \
  --from-file=uri=/secure/api-uri
# Repeat with the separate pipeline and migration files/Secret names above.
kubectl -n real-estate create secret generic backup-credentials \
  --from-file=ACCESS_KEY_ID=/secure/s3-access-key \
  --from-file=ACCESS_SECRET_KEY=/secure/s3-secret-key
```

If GHCR is private, create `ghcr-pull` from a protected dockerconfigjson file and
patch `real-estate-workload.imagePullSecrets` after configuration apply; persist
that non-secret reference in your release overlay. Do not put the registry token
in a ConfigMap. These external Secret references can later be managed by SOPS/age
or another secret controller without application changes.

## 3. Database, readiness, then migrations

```sh
make k8s-db-apply-single
make k8s-db-wait
kubectl -n real-estate get cluster real-estate-db -o yaml
kubectl -n real-estate get pods -l cnpg.io/cluster=real-estate-db -o wide
kubectl -n real-estate get service real-estate-db-rw
sh infrastructure/kubernetes/scripts/operator.sh apply single-node configuration
MIGRATION_JOB=$(sh infrastructure/kubernetes/scripts/operator.sh job single-node migrations)
kubectl -n real-estate logs -f "$MIGRATION_JOB"
kubectl -n real-estate wait --for=condition=Complete "$MIGRATION_JOB" --timeout=1800s
```

Stop on failure; inspect `kubectl describe`, logs and migration history.
The runner takes the pipeline's session advisory lock, applies sorted authoritative
`database/migrations/*.sql` followed by production privilege migrations, records
SHA-256 and applies each file/history entry atomically. Historical BEGIN/COMMIT
wrappers are removed in memory, not edited in the source. Already-applied files
are checksum-verified before any new file runs. Do not edit applied files: add a
new migration. A failed file leaves no partial file changes; earlier successful
files stay applied. A new uniquely named Job safely retries.

The runner deliberately refuses untracked existing databases: this is not an
automatic Compose-schema adoption tool. For existing development data, bootstrap
the fresh production schema then perform a reviewed data-only transfer with
constraints/row-count/publication validation, or rehearse a separately approved
history-baselining migration. Never invent history/checksums on live production.

The migration Job is the only app workload with superuser credentials. API startup
does not migrate. Production policy revokes API write/import/refresh privileges
while retaining the existing narrow auth/preferences/review writes. Pipeline has
ETN/analytics DML and temporary-table permissions, not schema creation or identity
table access. Fixed refresh functions use a trusted search_path and SECURITY
DEFINER so PostgreSQL16 ANALYZE can run without granting table ownership/schema
control to the pipeline. Future schema migrations must explicitly preserve these
grants/revocations; add versioned privilege migrations when new objects appear.

## 4. Initial data/bootstrap, deliberate model training

```sh
BOOTSTRAP_JOB=$(sh infrastructure/kubernetes/scripts/operator.sh job single-node bootstrap)
kubectl -n real-estate logs -f "$BOOTSTRAP_JOB"
kubectl -n real-estate wait --for=condition=Complete "$BOOTSTRAP_JOB" --timeout=24h
# This is a deliberate first training operation, not an application startup hook.
MODEL_JOB=$(sh infrastructure/kubernetes/scripts/operator.sh job single-node models-refresh)
kubectl -n real-estate logs -f "$MODEL_JOB"
kubectl -n real-estate wait --for=condition=Complete "$MODEL_JOB" --timeout=24h
```

Before opening public access, take the first on-demand base backup and verify
WAL upload using the [backup runbook](backup-and-recovery.md). Wait for completed
backup status; do not count a configured schedule as a successful backup.

Do not start the scheduler before bootstrap/jobs finish. Production sets
`MODEL_REFRESH_POLICY=manual`: imports and analytical publication refresh on the
existing 14-day schedule, but never silently trigger training, even after an empty
database reset. Until the explicit model Job completes, model-derived output can
be unavailable. Later new transactions may lack predictions until an explicit
training Job. Existing published model evidence is retained; it is not relabelled
as newly trained. Compose defaults to `auto` and keeps its existing first-install
and new-archive training behavior.

All manual maintenance commands now share the scheduler's existing PostgreSQL
advisory lock. Job retries are not automatic (`backoffLimit: 0`); inspect failure
before creating another Job. A graceful termination allows ten minutes for the
worker/job, but cannot promise an arbitrarily long calculation will finish before
the node stops. Database transactions protect interrupted changes.

## 5. API, web, ingress, then scheduler

```sh
sh infrastructure/kubernetes/scripts/operator.sh apply single-node api
kubectl -n real-estate rollout status deployment/real-estate-api --timeout=300s
sh infrastructure/kubernetes/scripts/operator.sh apply single-node web
kubectl -n real-estate rollout status deployment/web --timeout=300s
sh infrastructure/kubernetes/scripts/operator.sh apply single-node ingress
kubectl -n real-estate wait --for=condition=Ready certificate/real-estate-tls --timeout=600s
sh infrastructure/kubernetes/scripts/operator.sh apply single-node pipeline
kubectl -n real-estate rollout status deployment/pipeline-worker --timeout=24h
```

Check CNPG role status and authenticated API readiness, representative queries,
map/transaction UI, publication status, and logs before declaring service ready.
The worker readiness marker means a successful cycle has occurred in that pod,
not a permanent freshness guarantee. It has no liveness probe that kills a long
refresh. Monitor last successful publication and failed/retrying cycles.

The web image is built with empty `NEXT_PUBLIC_API_URL`: browser API requests use
same-origin `/api/v1`. Traefik sends `/api` to Go and `/` to Next.js. No runtime
NEXT_PUBLIC override is required or effective. Compose still builds its localhost
URL. Both images use the same code; each production image is reusable across
domains. Health endpoints are not separate public API ingress routes.

Let's Encrypt **staging** is the initial issuer (certificates are intentionally
untrusted by browsers). After verifying DNS, HTTP01 on port80, certificate status
and HTTPS routing, create a distinct `letsencrypt-production` Issuer with
`https://acme-v02.api.letsencrypt.org/directory` and a different account Secret,
then update the Ingress issuer annotation in the release overlay. Verify the new
certificate is browser-trusted. Do not reuse staging account Secrets. The
HTTP01-solver NetworkPolicy permits Traefik to reach temporary challenge pods.

## Subsequent application releases

Record/render the new SHA; verify backup, run the migration Job and wait, then
apply the appropriate app overlay and wait for each rollout. Do not rerun the
initial import or model Job. An API/web-only rollout can use their component
phases without replacing the worker. A worker restart immediately checks sources,
as before, but production policy prevents training. ConfigMap env changes take
effect on pod replacement: explicitly roll only the affected Deployment.

Database migrations must follow expand/contract compatibility when old and new
API versions overlap. `rollout undo` changes images, NOT database schema. A
breaking migration needs a separately rehearsed maintenance plan.

## One physical node → three physical nodes (same database architecture)

1. Verify a completed off-cluster PostgreSQL backup, WAL health and a restore test.
2. Verify a fresh off-node embedded-etcd snapshot and protected server token.
3. Provision node2 with private network/time/SSD prerequisites.
4. Provision node3 independently.
5. Join both as **K3s servers**, not agents, using the same pinned version, private
   registration endpoint, protected token file and matching critical settings:
   `sudo env INSTALL_K3S_VERSION='v1.35.8+k3s1' sh /tmp/install-k3s.sh server --server https://NODE1_PRIVATE:6443 --token-file /secure/k3s-server-token --secrets-encryption`.
   Do not pass `--cluster-init` to joiners. Move through the temporary two-member
   etcd phase promptly; two members do not tolerate one failure.
6. `kubectl get nodes -o wide`: three Ready control-plane/etcd nodes.
7. Verify etcd member list and endpoint health/quorum using a version-compatible
   etcdctl and the node's K3s etcd client certificates; check K3s service logs on
   all three. Ready nodes alone are not proof of healthy etcd membership.
8. Keep CNPG at one instance while checking the cluster.
9. Check `local-path` uses WaitForFirstConsumer, per-node disks/free space and
   node affinity on existing PVs. **Local storage != HA storage**.
10. `make k8s-db-apply-ha` changes the same Cluster to three instances.
11. Wait for all three database pods Ready and replicas streaming/replay caught up.
12. Verify CNPG status, primary, both replicas and successful WAL archiving/base backup.
13. Apply `kubectl apply -k infrastructure/kubernetes/addons/traefik/ha`;
    wait for two Ready Traefik replicas on different nodes. Then
    `make k8s-apps-apply-ha` (same images/host/Secrets).
14. Verify web/API each have two Ready replicas on different nodes.
15. Perform the controlled application drain test in the operations runbook.
16. Only **after synchronized replicas and a verified backup**, perform the
    PostgreSQL primary-node failover drill. Never kill the sole primary just after
    increasing instances.
17. Restore/uncordon the affected node.
18. Verify three Ready servers/etcd members, CNPG primary+two healthy replicas,
    web/API availability, pipeline recovery, PVC health and backups.

For step7, use a version-compatible etcdctl on a server with these verified K3s
certificate paths (the tool is operational, not an application dependency):

```sh
sudo etcdctl --endpoints=https://127.0.0.1:2379 \
  --cacert=/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt \
  --cert=/var/lib/rancher/k3s/server/tls/etcd/client.crt \
  --key=/var/lib/rancher/k3s/server/tls/etcd/client.key member list --write-out=table
sudo etcdctl --endpoints=https://127.0.0.1:2379 \
  --cacert=/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt \
  --cert=/var/lib/rancher/k3s/server/tls/etcd/client.crt \
  --key=/var/lib/rancher/k3s/server/tls/etcd/client.key endpoint health --cluster
```

CNPG owns streaming replication, promotion and replacement. The stable RW Service,
database schema, app images and connections do not change. Every DB instance has
its own PVC/node; PostgreSQL replication supplies database redundancy, not a shared
filesystem. Local PVCs do not teleport across failed nodes. Follow CNPG recovery
guidance for a permanently lost instance/PVC after recovering quorum; do not delete
the surviving primary's PVC. A later StorageClass change requires an operator-
supported rolling storage migration/restore, not simply editing bound PVC fields.

The single worker archive PVC remains node-local. Losing that node can pause
imports without taking web/API offline. Archives are reconstructable from GURS,
but plan a reviewed cache-volume replacement if the node is permanently lost.
Do not scale workers to bypass the storage issue.
