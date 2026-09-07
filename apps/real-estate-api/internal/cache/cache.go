// Package cache defines a future reconstructable-cache boundary.
// No handler uses Redis today. PostgreSQL remains the source of truth.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Store implementations must treat outages as misses, bound I/O by the caller's
// context/timeouts, and reject non-positive TTLs. An adapter must coalesce loads
// per key before handlers use it (including bounded cross-instance leases).
type Store interface {
	Get(context.Context, string) ([]byte, bool)
	Set(context.Context, string, []byte, time.Duration) error
}

// Disabled is the default; it has no network or availability dependency.
type Disabled struct{}

func (Disabled) Get(context.Context, string) ([]byte, bool) { return nil, false }
func (Disabled) Set(_ context.Context, _ string, _ []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("cache TTL must be positive")
	}
	return nil
}

// Key includes the published analytics release, model version and ALL normalized
// query inputs. Callers must not include credentials or sensitive user data.
func Key(namespace, release, model string, inputs any) (string, error) {
	payload, err := json.Marshal(struct {
		Release string
		Model   string
		Inputs  any
	}{release, model, inputs})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return "realestate:v1:" + namespace + ":" + hex.EncodeToString(digest[:]), nil
}
