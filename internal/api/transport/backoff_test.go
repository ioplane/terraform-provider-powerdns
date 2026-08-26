package transport

import (
	"math"
	"strconv"
	"testing"
	"time"
)

func TestBackoffDelayUsesMonotonicSaturatingIntegerArithmetic(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: -1, want: time.Second},
		{attempt: 0, want: time.Second},
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 3, want: 4 * time.Second},
		{attempt: 4, want: 8 * time.Second},
		{attempt: 5, want: 16 * time.Second},
		{attempt: 6, want: 16 * time.Second},
		{attempt: math.MaxInt, want: 16 * time.Second},
	} {
		if got := backoffDelay(test.attempt); got != test.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", test.attempt, got, test.want)
		}
	}

	previous := time.Duration(0)
	for attempt := 1; attempt <= 1_000; attempt++ {
		current := backoffDelay(attempt)
		if current < previous {
			t.Fatalf("attempt %d decreased from %v to %v", attempt, previous, current)
		}
		if current > backoffCap {
			t.Fatalf("attempt %d exceeded cap: %v", attempt, current)
		}
		previous = current
	}
}

func FuzzBackoffDelaySaturates(f *testing.F) {
	for _, attempt := range []int{1, 2, 5, 6, 63, math.MaxInt} {
		f.Add(attempt)
	}
	f.Fuzz(func(t *testing.T, attempt int) {
		if attempt < 1 {
			if delay := backoffDelay(attempt); delay != backoffBase {
				t.Fatalf("backoffDelay(%d) = %v, want base %v", attempt, delay, backoffBase)
			}
			return
		}
		delay := backoffDelay(attempt)
		if delay < backoffBase || delay > backoffCap {
			t.Fatalf("backoffDelay(%d) = %v outside [%v,%v]",
				attempt, delay, backoffBase, backoffCap)
		}
	})
}

func BenchmarkBackoffDelay(b *testing.B) {
	for _, attempt := range []int{1, 5, math.MaxInt} {
		b.Run(strconv.Itoa(attempt), func(b *testing.B) {
			for b.Loop() {
				_ = backoffDelay(attempt)
			}
		})
	}
}
