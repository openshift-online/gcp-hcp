package storage

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newShardTestObject(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.example.com/v1",
			"kind":       "TestObject",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func TestComputeObjectShard(t *testing.T) {
	t.Run("returns valid shard index", func(t *testing.T) {
		obj := newShardTestObject("default", "my-obj")
		shard, err := ComputeObjectShard(obj, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shard < 0 || shard >= 3 {
			t.Errorf("shard index %d out of range [0, 3)", shard)
		}
	})

	t.Run("deterministic for same object", func(t *testing.T) {
		obj := newShardTestObject("default", "my-obj")
		shard1, err := ComputeObjectShard(obj, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		shard2, err := ComputeObjectShard(obj, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shard1 != shard2 {
			t.Errorf("expected same shard, got %d and %d", shard1, shard2)
		}
	})

	t.Run("different objects can get different shards", func(t *testing.T) {
		shards := make(map[int]bool)
		for i := 0; i < 20; i++ {
			obj := newShardTestObject("default", fmt.Sprintf("obj-%d", i))
			shard, err := ComputeObjectShard(obj, 4)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			shards[shard] = true
		}
		if len(shards) < 2 {
			t.Errorf("expected objects to spread across shards, got only %d distinct shards", len(shards))
		}
	})

	t.Run("shard count of 1 always returns 0", func(t *testing.T) {
		obj := newShardTestObject("default", "my-obj")
		shard, err := ComputeObjectShard(obj, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shard != 0 {
			t.Errorf("expected shard 0, got %d", shard)
		}
	})
}

func TestMatchesShard(t *testing.T) {
	t.Run("nil selector matches everything", func(t *testing.T) {
		obj := newShardTestObject("default", "my-obj")
		matches, err := MatchesShard(obj, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Error("expected nil selector to match")
		}
	})

	t.Run("matches correct shard", func(t *testing.T) {
		obj := newShardTestObject("default", "my-obj")
		shard, err := ComputeObjectShard(obj, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		matches, err := MatchesShard(obj, &ShardSelector{Index: shard, Count: 3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !matches {
			t.Error("expected object to match its own shard")
		}
	})

	t.Run("does not match wrong shard", func(t *testing.T) {
		obj := newShardTestObject("default", "my-obj")
		shard, err := ComputeObjectShard(obj, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wrongShard := (shard + 1) % 3
		matches, err := MatchesShard(obj, &ShardSelector{Index: wrongShard, Count: 3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if matches {
			t.Errorf("expected object in shard %d to not match shard %d", shard, wrongShard)
		}
	})
}

func TestParseShardSelector(t *testing.T) {
	tests := []struct {
		name       string
		shardIndex string
		shardCount string
		wantIndex  int
		wantCount  int
		wantNil    bool
		wantErr    bool
	}{
		{
			name:       "both empty returns nil",
			shardIndex: "",
			shardCount: "",
			wantNil:    true,
		},
		{
			name:       "only index provided",
			shardIndex: "0",
			shardCount: "",
			wantErr:    true,
		},
		{
			name:       "only count provided",
			shardIndex: "",
			shardCount: "3",
			wantErr:    true,
		},
		{
			name:       "valid selector",
			shardIndex: "1",
			shardCount: "3",
			wantIndex:  1,
			wantCount:  3,
		},
		{
			name:       "index 0",
			shardIndex: "0",
			shardCount: "2",
			wantIndex:  0,
			wantCount:  2,
		},
		{
			name:       "invalid index",
			shardIndex: "abc",
			shardCount: "3",
			wantErr:    true,
		},
		{
			name:       "invalid count",
			shardIndex: "0",
			shardCount: "abc",
			wantErr:    true,
		},
		{
			name:       "negative index",
			shardIndex: "-1",
			shardCount: "3",
			wantErr:    true,
		},
		{
			name:       "zero count",
			shardIndex: "0",
			shardCount: "0",
			wantErr:    true,
		},
		{
			name:       "negative count",
			shardIndex: "0",
			shardCount: "-1",
			wantErr:    true,
		},
		{
			name:       "index equals count",
			shardIndex: "3",
			shardCount: "3",
			wantErr:    true,
		},
		{
			name:       "index greater than count",
			shardIndex: "5",
			shardCount: "3",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseShardSelector(tt.shardIndex, tt.shardCount)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseShardSelector() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil ShardSelector")
			}
			if got.Index != tt.wantIndex {
				t.Errorf("Index = %d, want %d", got.Index, tt.wantIndex)
			}
			if got.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", got.Count, tt.wantCount)
			}
		})
	}
}

func TestShardSelector_String(t *testing.T) {
	tests := []struct {
		name     string
		selector *ShardSelector
		want     string
	}{
		{
			name:     "nil selector",
			selector: nil,
			want:     "no shard",
		},
		{
			name:     "shard 0 of 3",
			selector: &ShardSelector{Index: 0, Count: 3},
			want:     "shard 0/3",
		},
		{
			name:     "shard 2 of 5",
			selector: &ShardSelector{Index: 2, Count: 5},
			want:     "shard 2/5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.selector.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
