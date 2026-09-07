# Kubernetes readiness report

Date: 2026-09-06. Baseline checked-out Git SHA:
`f1b1340514d3bc4f9e85e9979b8cbce252bbe0b8` (main).
This report describes the implementation in the working tree based on that SHA;
it does not imply these new files were already committed, pushed or deployed.

## Outcome and evidence boundary

The repository now has a single-node and three-node K3s deployment architecture
using **the same CloudNativePG database lifecycle from day one**. Local render,
strict CRD schema validation, client/server dry-runs in disposable K3s, fresh
production migrations, application tests and image builds have passed.

**This is repository/deployment readiness, not a claim of production testing.**
No real domain, certificate, object-store credentials, physical server, GHCR push,
CNPG replica failover or live backup/PITR was provisioned/executed. The production
application/operator workloads were not installed into a real production cluster.
The disposable K3s test installs CRDs, not CNPG/cert-manager/Barman controllers,
so operator admission-webhook/reconciliation behavior is a remaining deployment
gate, even though the Kubernetes API accepted the resources.

## Audit of current behavior and deliberate deviations

- Inspected existing Dockerfiles, Compose dependencies/health/resources, API
  startup/config/health/roles, pipeline scheduler/advisory locks, authoritative SQL,
  current migration runner and GitHub quality gates before implementation.
- Compose's migration runner assumes Docker init scripts have applied older
  migrations; it does not have atomic per-file checksum history. Preserved it for
  local development rather than breaking existing installations. Added a separate
  production runner/image that starts from a genuinely empty CNPG database.
- A **fourth infrastructure image**, migrations, is justified: schema ownership
  credentials and authoritative SQL are not put in API/web images. API, web and
  pipeline remain the three application images.
- Historical SQL names database/owner role real_estate and creates a development
  API password only when that role is missing. Production uses that bootstrap
  owner name and precreates missing roles NOLOGIN, avoiding the password fallback.
  CNPG reconciles real credentials separately; migration Jobs alone mount the
  dedicated postgres superuser Secret.
- PostgreSQL16 ANALYZE needs ownership. Production policy gives only fixed
  analytics refresh entry points SECURITY DEFINER with a trusted search_path,
  and revokes their API/PUBLIC execution. Pipeline keeps DML/TEMP/explicit refresh
  capability without schema-owner privileges. No statistical SQL definitions,
  ETN populations, model-selection definitions or API calculations were changed.
- Current scheduler trained on first import and whenever archives changed.
  Added validated MODEL_REFRESH_POLICY=auto|manual (auto default preserves Compose).
  Production manual policy requires explicit first and subsequent model Jobs.
  New analytical publications can therefore precede a deliberate model refresh;
  newly imported transactions may have no model prediction until then.
- Manual analytics/model CLI commands now use the same PostgreSQL lock as the
  scheduler/migration process. Locks are not replaced by Redis.
- Web already supports an empty built public API URL. GHCR production builds use
  it for same-origin /api/v1; Compose retains localhost build-time configuration.
  Next standalone packaging and numeric nonroot users were added; no UI or
  public API contract redesign was needed.
- Redis is not deployed or integrated. A no-op Go cache boundary and versioned key
  helper plus the reserved configuration/TTL/failure contract are useful preparation.
  No speculative Sentinel topology, unused client dependency or fake cache feature
  was added. CACHE_ENABLED=false remains fully functional; changing it alone does
  not enable a Redis client.

## Architecture and pins

| Item | Implemented value |
| --- | --- |
| Distribution | K3s v1.35.8+k3s1, embedded etcd from first node |
| Namespace | real-estate |
| Manifests/operator interface | Kustomize, kubectl, phase-specific POSIX-shell helpers |
| CNPG | 1.29.2 |
| Barman Cloud CNPG-I | 0.15.0, official plugin ObjectStore + Cluster.plugins |
| cert-manager | 1.20.3; staging issuer initially |
| PostgreSQL/PostGIS | 16.15 / 3.6.4, official CNPG system-trixie operand |
| DB image | ghcr.io/cloudnative-pg/postgis:16.15-3.6.4-202608311017-system-trixie |
| DB digest | sha256:9d597fcf4350e4bae9ff40cbe0e4383746a9c7f9c60da72483b0e966e4c20765 |
| Cluster | real-estate-db |
| Stable writable endpoint | real-estate-db-rw.real-estate.svc.cluster.local |
| Storage | local-path, 100Gi independent PVC per CNPG instance; worker archive PVC40Gi |
| Public entry | Traefik Ingress: / → web, /api → API; ClusterIP only |
| Image registry | GHCR SHA tags; amd64/arm64 publishing after main CI gates |
| Migration history | public.production_schema_migrations: filename, SHA256, applied_at |
| Durable source | PostgreSQL, not Redis or worker archive cache |

Pinned release sources are linked in infrastructure/kubernetes/addons/README.md.
Operators must deliberately upgrade compatible versions and check advisories;
these pins are not a promise of indefinite vendor/community support.

## Resources, security and topology

Single-node: web1, API1, worker1, CNPG1. HA: web2, API2, worker1, CNPG3.
HA web/API have hard topology spreading and minAvailable1 PDBs; CNPG required
hostname anti-affinity separates database instances and CNPG manages database
disruption/failover. Single-node contains no HA-only hard spreading constraint.
K3s HelmChartConfig additionally changes bundled Traefik from one to two replicas
with required node anti-affinity and a PDB; its upstream chart values were checked.
Default asynchronous PostgreSQL replication does not promise zero write-loss RPO.

Every long-running app/database container has requests/limits; see the exact
[resource table](kubernetes-operations.md#capacity-and-security). Initial limits:
API512Mi, web768Mi, worker4Gi, PostgreSQL4Gi each, Barman512Mi each. Worker/model
concurrency2 and math-library threads1 bound parallel CPU use. These starting
budgets still need representative capacity measurements and OS/addon headroom.

Nonroot IDs, no escalation, dropped capabilities, RuntimeDefault seccomp,
read-only app roots, writable bounded temporary/cache volumes and tokenless
ServiceAccounts are configured. CNPG pod security is operator-owned.
NetworkPolicies restrict namespace ingress to public app routing, temporary ACME
challenge routing, internal DB clients, replication and CNPG management traffic.
Egress allowlisting is deferred because GURS redirects/S3/DNS need real-provider
testing. No PostgreSQL/worker/management LoadBalancer, NodePort or Ingress exists.

ConfigMaps contain only non-sensitive settings. Database URIs/passwords, S3 and
optional private GHCR credentials reference externally created Secrets. DB clients
use verify-full TLS with the generated CNPG CA. K3s secret encryption/token custody
is documented. No production credentials were committed; test passwords only
exist in explicitly disposable test invocations, not production manifests.

## Lifecycle, backups and operations

Bootstrap is separately gated: prerequisites, namespace/secrets, CNPG Ready,
shared configuration, migration Job Complete, bootstrap Job Complete, deliberate
model Job, API rollout, web rollout, certificate, singleton scheduler. Subsequent
app rollouts do not replay bootstrap/model jobs.

Production migration files are ordered, checksum-verified, atomic with their
history entry and exclusive with pipeline work; insertion of an out-of-order
unapplied file is rejected. Applied-file changes and untracked existing DBs fail
closed. Failed earlier successful files remain committed; the failed file is
rolled back and retries skip verified completed files.

Database backups: daily 02:00 UTC base backup + WAL archiving to a private
off-cluster S3 prefix, Barman30d recovery window, Secret credentials. PITR/new-cluster
restore includes a distinct archive destination, private application validation
and controlled cutover. K3s embedded-etcd S3 snapshots and recovery are documented
independently, including the need for the original server token and credentials
when the Kubernetes API is unavailable.

The 1→3 runbook verifies both backup types, joins two K3s **server** nodes, verifies
etcd quorum/storage, expands the SAME CNPG Cluster, waits for streaming/caught-up
replicas, then applies HA app topology and performs controlled drains/failover.
Local PVCs are not distributed storage. Loss of the worker's local cache node can
pause refreshes; it does not require taking web/API offline.

## Verification executed

| Check | Actual result |
| --- | --- |
| make lint | PASS: pipeline Ruff, Go vet, frontend ESLint/typecheck |
| make test | PASS: full suite; final targeted Python re-run86 tests; Go race-enabled suite incl integration; frontend61 tests/12 files |
| Dedicated production migration tests | PASS: 3 tests, fresh databases; atomic rollback/retry, checksum drift, order, lock exclusion, extensions and role permissions |
| Production migration image executed | PASS: all30 authoritative SQL files + production policy on an empty disposable DB |
| make k8s-render-single / make k8s-render-ha | PASS |
| kubectl kustomize, both app and DB overlays | PASS |
| scripts/validate.py against pinned release CRDs | PASS: all11 Kustomize roots, strict custom-resource spec fields and topology/security invariants |
| scripts/smoke.py --existing | PASS: client AND server dry-runs for all11 roots against isolated K3s1.35.8 embedded-etcd |
| docker compose config --quiet | PASS |
| docker compose build real-estate-api pipeline-worker web | PASS |
| Same-origin production web build | PASS: empty NEXT_PUBLIC_API_URL |
| Dedicated migrations image build | PASS |
| Actual pinned PG16/PostGIS operand | PASS: pulled digest, PostgreSQL16.15, CREATE EXTENSION postgis/pgcrypto; PostGIS reports3.6.4 |
| Read-only/nonroot production web runtime | PASS: UID1000, dropped capabilities, HTTP / served |
| Read-only pipeline entrypoint | PASS: CLI starts without writable app root |
| Production-mode API runtime | PASS: restricted API identity, health and empty/suppressed statistics query |
| DB outage behavior | PASS: liveness200/readiness503 while ONLY disposable DB stopped; readiness/query recovered after DB restart without API restart |
| Shell syntax, extra tooling Ruff, workflow YAML parsing, git diff --check | PASS |
| Bootstrap configuration isolation | PASS: configuration phase creates account/config/PVC without launching any application Deployment |

Tests used isolated containers named real-estate-readiness-* for infrastructure
validation. Existing standard make integration tests used their configured test
database and self-cleaning fixtures. No full historical import, analytics rebuild
or model training was required for these readiness checks. Initial local socket
permission failures were rerun with the required sandbox permission; they are not
unresolved application failures.

Repeatable schema/dry-run commands:

```sh
pip install -r infrastructure/kubernetes/scripts/requirements.txt
. infrastructure/kubernetes/addons/versions.env
curl -fsSL "$CNPG_MANIFEST" -o /tmp/cnpg.yaml
curl -fsSL "$BARMAN_MANIFEST" -o /tmp/barman.yaml
python infrastructure/kubernetes/scripts/validate.py /tmp/cnpg.yaml /tmp/barman.yaml
# Docker-backed disposable test cluster; destroys only its own fixed test container.
python infrastructure/kubernetes/scripts/smoke.py
make lint
make test
git diff --check
```

The smoke script runs `kubectl apply --dry-run=client` and
`kubectl apply --dry-run=server` using its own ephemeral kubeconfig. It never
targets the operator's normal context. It also runs in CI. The image-publish job
depends on all quality gates; it was added but has NOT been executed on GitHub in
this local session.

## Remaining production gates and deliberately deferred work

Before serving users:

1. Supply real nodes, capacity, private registration endpoint/firewall, public
   hostname/DNS, protected Secrets and verified off-cluster S3 credentials.
2. Deploy the pinned controllers and actual Cluster; verify managed-role
   reconciliation, PVC topology, CA/TLS connections and NetworkPolicy enforcement.
3. Perform real WAL/base backup/PITR and independent etcd restore tests. Measure
   actual RPO/RTO; local schema validation cannot demonstrate recoverability.
4. Issue staging then trusted production certificates, verify renewal and ingress.
   HA ingress also needs a deliberate external routing strategy to surviving node
   IPs (provider load balancer/health-aware routing); K3s pod replication alone
   cannot fail over an Internet connection pinned to a failed server address.
5. Verify GHCR publication/visibility and actual image digests; protect SHA tags
   from overwrite, or pin operator release overlays to resolved digests.
6. Rehearse three-physical-node etcd/CNPG expansion, application drains and primary
   failure. Test zero/limited write-loss requirements against the chosen replication
   policy. This was not simulated by multiple database pods on one machine.
7. Capacity-test representative GURS/analytics/model workloads and alerting;
   explicit limits are not measured performance guarantees.

Deferred as requested: Terraform/OpenTofu/Ansible, Rancher/ArgoCD installation,
Prometheus/Grafana/Loki/Alloy installation, VPN/DNS/firewall automation, distributed
storage, service mesh, Kafka, HPA and Redis sharding/real cache feature. Future
Prometheus discovery remains compatible with CNPG; application business metrics
still need instrumentation. Redis Sentinel requires a deliberate maintained
deployment/client choice and writable configuration; none was improvised here.

The deployment architecture invariant is satisfied in code: moving from one to
three physical servers changes K3s/etcd/CNPG/app topology, not the application
boundaries or database deployment architecture. Real HA/recovery acceptance still
requires the physical-environment procedures above.
