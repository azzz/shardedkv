package chooser

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
)

type uint64Slice []uint64

func (x uint64Slice) Len() int           { return len(x) }
func (x uint64Slice) Less(i, j int) bool { return x[i] < x[j] }
func (x uint64Slice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// HashRing uses Consistent Hash Ring to choose a worker node
type HashRing struct {
	mu       sync.Mutex
	replicas int               // number of virtual replicas per shard
	hashes   uint64Slice       // ordered slice of hashes of both real and virtual shards
	shards   map[uint64]string // hash-to-shard map
	hashfn   func([]byte) (uint64, error)
}

func New(replicas int) *HashRing {
	return &HashRing{
		mu:       sync.Mutex{},
		replicas: replicas,
		hashes:   uint64Slice{},
		shards:   map[uint64]string{},
		hashfn:   hash,
	}
}

// Add shard to the ring with `h.replicas` virtual shards.
func (h *HashRing) Add(shard string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := 0; i < h.replicas; i++ {
		sh, err := h.hashfn([]byte(strconv.Itoa(i) + shard))
		if err != nil {
			return fmt.Errorf("failed to hash shard: %w", err)
		}

		h.hashes = append(h.hashes, sh)
		h.shards[sh] = shard
	}

	sort.Sort(h.hashes)

	return nil
}

// Get a shard by a key.
func (h *HashRing) Get(key string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.hashes) == 0 {
		return "", nil
	}

	kh, err := h.hashfn([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to hash key: %w", err)
	}

	sh := searchRanges(h.hashes, kh)
	return h.shards[sh], nil
}

// return the closest value from the top. Return the first value if there is no segment ending
// The given slice must be ordered.
// Given arr: [4,8,15,16,23,42]
// Looking for 3, returning 4 as (0...4] segment includes 3
// Looking for 4, returning 4 as (0...4] segment includes 4
// Looking for 9, returning 15 as (8...15] segment includes 15.
// Looking for 45, returning 4 as (42...42] segment does not include 45.
func searchRanges(arr []uint64, val uint64) uint64 {
	if len(arr) == 0 {
		return 0
	}

	var bottom uint64 = 0

	for _, top := range arr {
		if val > bottom && val <= top {
			return top
		}
	}

	return arr[0]
}
