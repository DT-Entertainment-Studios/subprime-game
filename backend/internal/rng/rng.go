// Package rng is the game's randomness source.
//
// The production source reads crypto/rand directly for every draw. A failure
// of the system entropy source panics: there is no safe fallback. Tests may
// inject a deterministic source through the Random interface.
package rng

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Random is the randomness the game needs: uniform values in [0,n) and a
// Fisher-Yates shuffle. *math/rand.Rand satisfies it, so tests can inject a
// seeded deterministic source. Production uses crypto/rand.
type Random interface {
	Intn(n int) int
	Shuffle(n int, swap func(i, j int))
}

// Crypto is the production Random. Every draw reads crypto/rand directly.
type Crypto struct{}

// New returns the production randomness backed by crypto/rand.
func New() Random { return Crypto{} }

// Intn returns a uniform value in [0,n). It panics for n <= 0 and when the
// system entropy source fails: there is no safe fallback.
func (Crypto) Intn(n int) int {
	if n <= 0 {
		panic("rng: invalid argument to Intn")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(fmt.Errorf("rng: crypto/rand failed: %w", err))
	}
	return int(v.Int64())
}

// Shuffle runs a Fisher-Yates shuffle over n elements.
func (s Crypto) Shuffle(n int, swap func(i, j int)) { shuffle(s, n, swap) }

// intner is the single draw shuffle needs; it keeps the loop testable without
// real entropy.
type intner interface{ Intn(n int) int }

func shuffle(random intner, n int, swap func(i, j int)) {
	if n < 0 {
		panic("rng: invalid argument to Shuffle")
	}
	for i := n - 1; i > 0; i-- {
		swap(i, random.Intn(i+1))
	}
}
