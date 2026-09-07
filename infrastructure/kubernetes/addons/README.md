# Pinned prerequisites, not application resources

Pins live in `versions.env`: K3s **v1.35.8+k3s1**, CNPG **1.29.2**,
Barman Cloud CNPG-I **0.15.0**, cert-manager **1.20.3**. This is a deliberately
compatible Kubernetes 1.35 set, not automatic tracking of newest releases.
Confirm security advisories and supported release windows before every install.

Install cert-manager first, then CNPG, then Barman **in cnpg-system alongside
CNPG**. Use the runbook's Deployment availability and CRD establishment gates.
No upstream platform manifests are vendored. The application does not manage the
operator lifecycle or override CNPG-generated PostgreSQL pod security.

Traefik uses the K3s-bundled chart40.1.4+up40.1.0 / Traefik3.7.8.
`traefik/base` supplies single-node resources; `traefik/ha` sets two replicas,
required node anti-affinity and a PDB through K3s HelmChartConfig, not a second
Ingress-controller installation. Values were checked against the bundled upstream
chart40.1.0. Do not edit K3s' generated traefik.yaml. Merge any existing
HelmChartConfig customization deliberately rather than overwriting operator settings.

CNPG 1.29 supports the selected native-sidecar Barman plugin (plugin minimum
CNPG 1.26); the plugin uses cert-manager-issued internal certificates.
PostGIS is included in the official pinned PG16 operand, not a PG18 image-volume
extension or the incompatible Compose image.

Before upgrades: verify backups AND restore test, inspect release notes/CRD
changes, test on a disposable cluster, upgrade operator/plugin/cert-manager
deliberately and validate a backup/recovery cycle. Application releases do not
silently upgrade operators. Pin image digests and archive the rendered release.

Sources:

- [CNPG installation](https://cloudnative-pg.io/docs/1.29/installation_upgrade/)
- [CNPG supported releases](https://cloudnative-pg.io/docs/1.29/supported_releases/)
- [PostGIS operand support](https://cloudnative-pg.io/docs/1.29/postgis/)
- [Barman plugin installation](https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/)
- [cert-manager 1.20 releases](https://cert-manager.io/docs/releases/release-notes/release-notes-1.20/)
- [K3s v1.35.8](https://github.com/k3s-io/k3s/releases/tag/v1.35.8%2Bk3s1)

## Future management and observability

Rancher (cluster/workload UI), Argo CD (GitOps), Prometheus (metrics), Grafana
(dashboards), Loki with Grafana Alloy (logs) are **not installed**. They remain
management/observability layers, not runtime dependencies. No deprecated logging
collector is introduced. Standard labels and ClusterIP Services provide discovery
boundaries; CNPG retains its supported Prometheus metrics endpoint/integration.
No ServiceMonitor is emitted without its CRD and no fake application /metrics
endpoint is provided.

Add monitoring for node/disk/pod health, resource saturation, API latency/errors,
DB connections, primary/replica state and replication lag, backup freshness and
WAL failures, publication freshness/pipeline last success. Redis hit rate, memory,
evictions and Sentinel failover matter only once Redis actually exists.
API/pipeline business metrics still require instrumentation; logs and published
refresh-status data are the present evidence. `kubectl top` depends on K3s'
bundled metrics-server being ready; it is not historical monitoring.
