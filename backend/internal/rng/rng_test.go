package rng

import (
	mrand "math/rand"
	"testing"
)

// Compile-time proof of the test contract: *math/rand.Rand is a Random.
var _ Random = (*mrand.Rand)(nil)

func TestCryptoIntnBounds(t *testing.T) {
	random := New()
	for i := 0; i < 200; i++ {
		for n := 1; n <= 8; n++ {
			got := random.Intn(n)
			if got < 0 || got >= n {
				t.Fatalf("Intn(%d) = %d, want in [0,%d)", n, got, n)
			}
		}
	}
	if got := random.Intn(1); got != 0 {
		t.Fatalf("Intn(1) = %d, want 0", got)
	}
}

func TestCryptoIntnPanicsOnNonPositive(t *testing.T) {
	for _, n := range []int{0, -1} {
		assertPanics(t, func() { New().Intn(n) })
	}
}

func TestCryptoShuffleIsPermutation(t *testing.T) {
	random := New()
	for i := 0; i < 20; i++ {
		got := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		random.Shuffle(len(got), func(a, b int) { got[a], got[b] = got[b], got[a] })
		if len(got) != 10 {
			t.Fatalf("shuffled length = %d, want 10", len(got))
		}
		counts := make(map[int]int, len(got))
		for _, v := range got {
			counts[v]++
		}
		for v := 0; v < 10; v++ {
			if counts[v] != 1 {
				t.Fatalf("value %d appears %d times in %v, want once", v, counts[v], got)
			}
		}
	}
}

func TestShuffleMatchesFisherYates(t *testing.T) {
	random := &scriptedIntner{t: t, vals: []int{5, 0, 4, 1, 0}}
	got := []int{0, 1, 2, 3, 4, 5}
	type call struct{ i, j int }
	var calls []call
	shuffle(random, len(got), func(i, j int) {
		calls = append(calls, call{i, j})
		got[i], got[j] = got[j], got[i]
	})

	want := []call{{5, 5}, {4, 0}, {3, 4}, {2, 1}, {1, 0}}
	if len(calls) != len(want) {
		t.Fatalf("swap calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("swap call %d = %v, want %v", i, calls[i], want[i])
		}
	}
	wantSlice := []int{2, 4, 1, 0, 3, 5}
	for i := range wantSlice {
		if got[i] != wantSlice[i] {
			t.Fatalf("shuffled slice = %v, want %v", got, wantSlice)
		}
	}
}

func TestShuffleNoSwapsBelowTwo(t *testing.T) {
	for _, n := range []int{0, 1} {
		random := &fatalIntner{t: t}
		swaps := 0
		shuffle(random, n, func(i, j int) { swaps++ })
		if swaps != 0 {
			t.Fatalf("shuffle(%d) made %d swaps, want 0", n, swaps)
		}
	}
}

func TestShufflePanicsOnNegative(t *testing.T) {
	assertPanics(t, func() { New().Shuffle(-1, func(i, j int) {}) })
}

func assertPanics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic, got none")
		}
	}()
	f()
}

// scriptedIntner returns a fixed sequence of draws and fails the test when the
// sequence runs out.
type scriptedIntner struct {
	t    *testing.T
	vals []int
	pos  int
}

func (s *scriptedIntner) Intn(n int) int {
	s.t.Helper()
	if s.pos >= len(s.vals) {
		s.t.Fatalf("unexpected extra Intn(%d) call", n)
	}
	v := s.vals[s.pos]
	s.pos++
	return v
}

// fatalIntner fails the test on any draw.
type fatalIntner struct {
	t *testing.T
}

func (f *fatalIntner) Intn(n int) int {
	f.t.Helper()
	f.t.Fatalf("unexpected Intn(%d) call", n)
	return 0
}
