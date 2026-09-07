# Optional reconstructable cache: not deployed

The application works without Redis. `CACHE_ENABLED=false` is set in production.
The Go `internal/cache` package defines a disabled store and versioned-key
boundary only; **no Redis client, live cache feature, Sentinel manifest or Redis
startup dependency is implemented**. Setting a Redis environment variable does
not enable a cache. Implement and test a concrete first use case before enabling.

Reserved configuration contract (non-secrets in ConfigMaps):

| Setting | Future default/meaning |
| --- | --- |
| CACHE_ENABLED | false |
| REDIS_MODE | standalone; later sentinel; cluster only if sharding is measured necessary |
| REDIS_URL | private standalone endpoint, never a public address |
| REDIS_SENTINEL_MASTER | Sentinel master name |
| REDIS_SENTINEL_ADDRESSES | three private Sentinel endpoints |
| REDIS_USERNAME / REDIS_PASSWORD | Secret-backed ACL credentials |
| REDIS_CONNECT_TIMEOUT | 500ms |
| REDIS_READ_TIMEOUT / REDIS_WRITE_TIMEOUT | 200ms each, bounded by request context |

Evolution: no cache → optional standalone → primary + two replicas and three
Sentinels on three independent nodes → Cluster only if sharding is necessary.
Choose a maintained controller/chart at implementation time. Sentinel must have
writable runtime configuration/state, not a read-only ConfigMap. Clients must
support Sentinel discovery and reconnect to the promoted primary; never hard-code
the primary or implement custom failover. Three Sentinels are required for quorum.

Keys: `realestate:v1:{statistics|geography|building|valuation-context}:<hash>`.
Hash inputs include the active analytical release, model version, transaction
kind, property group, dates/frequency, inclusive area bounds, geography/bbox/zoom,
population/publication settings, pagination and any authorization scope.
Normalize equivalent queries before hashing. Never share private-user results.
The helper includes release, model and query inputs but callers remain responsible
for supplying **all** relevant inputs.

Proposed bounded TTLs: statistics 5m, geography 1h, building 5m, valuation context
2m. Add jitter and bounded per-key request coalescing/leases to prevent stampedes.
An unavailable Redis returns a miss and PostgreSQL serves the normal request.
Use short timeouts/circuit breaking and capacity limits to prevent a cache outage
from overwhelming PostgreSQL. New publication generations invalidate keys without
`FLUSHALL`. Never cache pending model releases as published evidence.

When deployed, configure explicit requests/limits, `maxmemory` below the container
limit (allow overhead/replication), and a measured eviction policy such as
`allkeys-lfu`. For reconstructable data, RDB/AOF and PVCs are optional, not default
durability requirements. ETN/RPE and user data stay in PostgreSQL; the PostgreSQL
pipeline advisory lock stays in PostgreSQL. Durable operational state in Redis
would require a separate architecture/recovery decision.

Future Sentinel test: verify quorum, identify current primary, fail its node,
observe promotion and Sentinel-aware client reconnection, restore the node and
verify replication. Asynchronous replication does not guarantee zero acknowledged
write loss. No Redis operations commands are provided because no resources exist.

Reference: [Redis Sentinel documentation](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/).
