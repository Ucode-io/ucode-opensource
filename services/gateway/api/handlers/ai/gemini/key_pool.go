package gemini

import (
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// keyCooldown is how long a key stays inactive after a 429.
// Gemini free-tier resets per minute; 62s guarantees we're past the window.
const keyCooldown = 62 * time.Second

// ErrAllKeysRateLimited is returned when every key in the pool is on cooldown.
// Callers should surface this as a user-visible "try again later" message.
var ErrAllKeysRateLimited = errors.New("все API ключи Gemini временно исчерпаны — попробуйте через минуту")

// KeyPool manages a set of API keys with round-robin selection and per-key
// rate-limit cooldowns. All methods are safe for concurrent use.
type KeyPool struct {
	keys      []string
	coolUntil []atomic.Int64 // Unix nanosecond timestamp: key available when now >= coolUntil[i]
	next      atomic.Uint64  // round-robin slot counter
}

// NewKeyPool returns a KeyPool for the given keys.
// Returns an error if keys is empty.
func NewKeyPool(keys []string) (*KeyPool, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("gemini.NewKeyPool: at least one API key is required")
	}
	return &KeyPool{
		keys:      keys,
		coolUntil: make([]atomic.Int64, len(keys)),
	}, nil
}

// pick returns the next available key and its index.
// It atomically reserves a unique starting slot (round-robin) then scans
// forward to find the first key not on cooldown.
// Returns ErrAllKeysRateLimited immediately if every key is cooling — never blocks.
func (p *KeyPool) pick() (string, int, error) {
	n := len(p.keys)

	// Reserve a unique slot for this goroutine; other concurrent callers get different slots.
	slot := int(p.next.Add(1)-1) % n

	// Linear scan from our slot: first available key wins.
	for i := range n {
		idx := (slot + i) % n
		if p.coolUntil[idx].Load() <= time.Now().UnixNano() {
			return p.keys[idx], idx, nil
		}
	}

	// All keys on cooldown — log when cooldown expires and fail fast.
	earliest := p.coolUntil[0].Load()
	for i := 1; i < n; i++ {
		if cd := p.coolUntil[i].Load(); cd < earliest {
			earliest = cd
		}
	}
	retryIn := time.Duration(earliest - time.Now().UnixNano()).Round(time.Second)
	log.Printf("[GEMINI POOL] all %d keys rate-limited, earliest retry in %v", n, retryIn)
	return "", 0, ErrAllKeysRateLimited
}

// markRateLimited puts key[idx] on cooldown for keyCooldown duration.
func (p *KeyPool) markRateLimited(idx int) {
	p.coolUntil[idx].Store(time.Now().Add(keyCooldown).UnixNano())
	log.Printf("[GEMINI POOL] key[%d] rate-limited, cooling for %v", idx, keyCooldown)
}
