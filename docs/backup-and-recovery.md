# Backup and recovery

**etcd backup != PostgreSQL backup. PostgreSQL backup != etcd backup. Both are
required.** A replica is also not a backup. One physical server plus local PVCs
has recoverability, not physical-node availability.

## PostgreSQL architecture and schedule

CNPG 1.29.2 manages real-estate-db. Barman Cloud CNPG-I 0.15.0 adds the supported
WAL archiver/backup sidecar. No new deprecated built-in barmanObjectStore
configuration is used.

`ObjectStore/real-estate-backups` targets a private, **off-cluster** S3-compatible
bucket/prefix with TLS and Secret-backed credentials. Choose a different failure
domain from all database disks. Limit credential access to that prefix; Barman
retention needs appropriate list/read/write/delete permissions. Enable encryption
at rest at the provider and retain any required encryption keys independently.

`ScheduledBackup/real-estate-daily` requests a base backup daily at 02:00 UTC
(six-field CNPG schedule `0 0 2 * * *`). Continuous WAL archiving plus those base
backups supports point-in-time recovery. RetentionPolicy30d is a **recovery
window**, not “exactly thirty backup files.” Retain needed WAL/base backups and
allow margin when configuring object-store lifecycle rules; never expire objects
earlier than Barman's recovery window. Rehearse retention and recovery with the
actual provider. Choose immutable/offline protection with a reviewed retention
policy; object locking can prevent Barman deletions and needs deliberate design.

PostgreSQL archive_timeout300s bounds WAL-segment switch time under light writes,
not total RPO: upload failures/network backlogs still increase loss exposure.
RTO/RPO are unmeasured until a restore drill. Monitor archive failures, last
successful upload, backup freshness and disk utilization (failed archives can
fill pg_wal). WAL is not useful without a compatible base backup.

Sources: [plugin usage/recovery](https://cloudnative-pg.io/plugin-barman-cloud/docs/usage/),
[retention](https://cloudnative-pg.io/plugin-barman-cloud/docs/retention/),
[CNPG recovery](https://cloudnative-pg.io/docs/1.29/recovery/).

## Verify WAL and take an on-demand backup

```sh
PRIMARY=$(kubectl -n real-estate get cluster real-estate-db -o jsonpath='{.status.currentPrimary}')
kubectl -n real-estate exec "$PRIMARY" -c postgres -- psql -U postgres -d real_estate -c \
  "SELECT archived_count,last_archived_wal,last_archived_time,failed_count,last_failed_time FROM pg_stat_archiver"
kubectl -n real-estate exec "$PRIMARY" -c postgres -- psql -U postgres -d real_estate -c "SELECT pg_switch_wal()"
kubectl -n real-estate logs "$PRIMARY" -c plugin-barman-cloud --since=30m
kubectl -n real-estate get scheduledbackup real-estate-daily -o yaml
```

Wait for archived WAL progress after the switch, confirm objects appear in the
off-node store, and inspect logs/status rather than assuming exit0 proves upload.
Use this manifest saved outside generated release artifacts as backup-request.yaml:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata:
  generateName: real-estate-manual-
  namespace: real-estate
spec:
  cluster:
    name: real-estate-db
  method: plugin
  pluginConfiguration:
    name: barman-cloud.cloudnative-pg.io
```

```sh
BACKUP=$(kubectl create -f backup-request.yaml -o name)
kubectl -n real-estate wait --for=jsonpath='{.status.phase}'=completed "$BACKUP" --timeout=2h
kubectl -n real-estate get "$BACKUP" -o yaml
```

Verify end time, backup identifier and object-store content. Failed jobs/backups
must not be counted as successful. CNPG objects may be absent after a cluster
disaster: preserve the object-store prefix/serverName and backup inventory
independently of etcd, and verify recoverability from that inventory.

## Restore to a NEW cluster, including PITR

Never test restore by overwriting real-estate-db or its PVCs.

1. Stop writers only if performing a real incident cutover; routine restore tests
   can run alongside production within independent resource/storage capacity.
2. Install the pinned operators in the recovery cluster if necessary. Restore
   credentials securely. Provide source ObjectStore real-estate-backups with the
   exact existing backup prefix and read access. Do not create an empty substitute.
3. Choose an explicit UTC recovery time before the unwanted change, within the
   verified backup/WAL recovery window. Latest recovery omits recoveryTarget.
4. Create a **different** writable ObjectStore, real-estate-restore-backups, with
   a separate empty prefix (for example s3://BUCKET/restore-drill-20260906).
   Reuse the source store's credential references only if they are authorized
   for the new prefix. Never reuse the production archive destination for writes.
5. Create a new Cluster:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: real-estate-restore
  namespace: real-estate
spec:
  instances: 1
  imageName: ghcr.io/cloudnative-pg/postgis:16.15-3.6.4-202608311017-system-trixie@sha256:9d597fcf4350e4bae9ff40cbe0e4383746a9c7f9c60da72483b0e966e4c20765
  storage:
    storageClass: local-path
    size: 100Gi
  resources:
    requests: {cpu: 500m, memory: 2Gi}
    limits: {cpu: "2", memory: 4Gi}
  bootstrap:
    recovery:
      source: original
      recoveryTarget:
        targetTime: "2026-09-06T10:00:00Z" # EXAMPLE: replace with the chosen incident target
  externalClusters:
    - name: original
      plugin:
        name: barman-cloud.cloudnative-pg.io
        parameters:
          barmanObjectName: real-estate-backups
          serverName: real-estate-db
  plugins:
    - name: barman-cloud.cloudnative-pg.io
      isWALArchiver: true
      parameters:
        barmanObjectName: real-estate-restore-backups
```

Add a copy of the internal PostgreSQL NetworkPolicy scoped to
`cnpg.io/cluster: real-estate-restore` (both selector and replication source).
The namespace's default-deny policy otherwise blocks the operator. Do not add
public ingress. Restore role/superuser management fields from the original
Cluster if remote password access is required; ensure CNPG reconciles intended
credentials and mount the **restored cluster's** CA for verification clients.

```sh
kubectl apply -f restore-cluster.yaml
kubectl -n real-estate wait --for=condition=Ready cluster/real-estate-restore --timeout=2h
kubectl -n real-estate get cluster real-estate-restore -o yaml
kubectl -n real-estate get pods -l cnpg.io/cluster=real-estate-restore -o wide
```

If the target is unreachable/missing WAL, investigate; do not silently restore a
different timestamp and claim PITR succeeded. Record target and actual recovery
evidence in the incident report. Once promoted, test:

- PostGIS/pgcrypto installed; expected PG16/PostGIS versions.
- Migration checksums/history match the chosen application release.
- Raw/source counts and transaction dates match the selected restore point.
- Active analytical/model publications and prediction references are consistent.
- API identity cannot import/migrate; pipeline identity remains separate.
- A private test API connected to real-estate-restore-rw serves health, overview,
  map, transaction, comparables, source/publication status, auth/preferences.
- Browser same-origin flow with a private test route, without accidentally writing
  to production. Check intended before-target data exists and after-target
  sentinel data is absent.
- New restored-cluster WAL archiving and a full base backup succeed to its OWN
  prefix.

For an incident cutover, freeze imports/manual jobs and writes, capture final
evidence/backups if possible, then update client Secret URIs and mounted CA to the
**restored CNPG stable RW Service**, roll clients, and verify before opening writes.
Never point clients at a pod. Do not remove the old Cluster/PVCs until rollback
retention and operator approval are satisfied. After recovery, establish the
three-instance topology through CNPG and wait for replicas before any failover.

## Restore-test checklist

Perform at least monthly and after database/operator/plugin/storage changes:

- [ ] Source base backup and WAL inventory available off-cluster.
- [ ] Independent recovery environment and sufficient disk/RAM.
- [ ] Secrets/encryption keys recoverable without the failed cluster.
- [ ] Explicit target time and before/after test records.
- [ ] New Cluster and distinct archive prefix; production unmodified.
- [ ] Target reached; schema, source rows, analytics and model publications validated.
- [ ] API least privilege and user-facing query checks passed.
- [ ] Restored WAL/base backup verified.
- [ ] Measured recovery duration/RPO, errors and operator actions recorded.
- [ ] Cleanup separately approved; original backup retention preserved.

## K3s embedded etcd is a separate backup

etcd stores Kubernetes objects/secrets/cluster state, **not PostgreSQL PVC data**.
PostgreSQL Barman backups do not contain the K3s control-plane datastore.

On each K3s server configure encrypted secrets and embedded-etcd snapshots in
/etc/rancher/k3s/config.yaml. Example non-secret settings:

```yaml
secrets-encryption: true
etcd-snapshot-schedule-cron: "0 */6 * * *"
etcd-snapshot-retention: 14
etcd-s3: true
etcd-s3-config-secret: k3s-etcd-s3
etcd-s3-retention: 14
```

Create the kube-system Secret k3s-etcd-s3 (type etcd.k3s.cattle.io/s3-config-secret)
through protected files with keys etcd-s3-access-key, etcd-s3-secret-key,
etcd-s3-endpoint, etcd-s3-bucket, etcd-s3-region and etcd-s3-folder.
Use a separate prefix/credential scope from PostgreSQL. Restart K3s deliberately
to apply server configuration; don't restart all HA servers simultaneously.

```sh
sudo k3s etcd-snapshot save --name pre-change
sudo k3s etcd-snapshot ls
sudo k3s etcd-snapshot list
kubectl get etcdsnapshotfiles
```

Confirm the object is off-node, not merely a local snapshot. Preserve the original
server token and relevant server configuration/certificates in independent secure
custody. Secret-based S3 configuration cannot be read when the Kubernetes API is
down: supply S3 credentials via a protected restore configuration/environment, not
shell history, following the pinned K3s recovery instructions.

Disaster restoration is explicitly destructive control-plane maintenance:
stop K3s on servers, select a verified snapshot and original token, restore on one
designated server with the documented `k3s server --cluster-reset
--cluster-reset-restore-path=...` procedure, restart it normally, then rejoin the
other servers according to the K3s runbook. Do not execute cluster-reset during
ordinary deployment. Snapshot restoration alone cannot resurrect lost local DB
volumes: recover PostgreSQL through Barman separately if necessary.

After etcd recovery verify nodes, quorum, CRDs/operators, Secrets/CA references,
CNPG/PVC state, ingress/certificates, backups and app readiness. Restored desired
state may restart a worker: pause it until DB/publication consistency is verified.
Test etcd recovery independently from PostgreSQL restoration.

Sources: [K3s snapshot/S3/restore documentation](https://docs.k3s.io/cli/etcd-snapshot),
[embedded-etcd HA](https://docs.k3s.io/datastore/ha-embedded).
