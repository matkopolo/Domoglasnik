package cache

import (
	"context"
	"testing"
	"time"
)

func TestDisabledHasNoDependency(t *testing.T) {
	store := Disabled{}
	if _, hit := store.Get(context.Background(), "missing"); hit {
		t.Fatal("disabled cache must miss")
	}
	if err := store.Set(context.Background(), "k", nil, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "k", nil, 0); err == nil {
		t.Fatal("unbounded TTL accepted")
	}
}

func TestKeyIncludesVersionsAndInputs(t *testing.T) {
	first, _ := Key("statistics", "release-1", "model-1", map[string]int{"from": 2024})
	repeat, _ := Key("statistics", "release-1", "model-1", map[string]int{"from": 2024})
	if first != repeat {
		t.Fatal("key must be deterministic")
	}
	for _, args := range [][3]string{{"release-2", "model-1", "2024"}, {"release-1", "model-2", "2024"}} {
		key, _ := Key("statistics", args[0], args[1], map[string]int{"from": 2024})
		if key == first {
			t.Fatal("generation not incorporated")
		}
	}
	changed, _ := Key("statistics", "release-1", "model-1", map[string]int{"from": 2025})
	if changed == first {
		t.Fatal("filter not incorporated")
	}
}
