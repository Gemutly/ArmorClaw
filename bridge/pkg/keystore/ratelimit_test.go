package keystore

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterExceeded(t *testing.T) {
	rl := NewRateLimiter(3, 60*time.Second)

	for i := 0; i < 3; i++ {
		rl.Record("alice")
	}
	if rl.Exceeded("alice") {
		t.Fatal("should not be exceeded at exactly maxAttempts")
	}

	rl.Record("alice")
	if !rl.Exceeded("alice") {
		t.Fatal("should be exceeded after maxAttempts+1")
	}
}

func TestRateLimiterPerIdentity(t *testing.T) {
	rl := NewRateLimiter(2, 60*time.Second)

	for i := 0; i < 5; i++ {
		rl.Record("alice")
	}
	if !rl.Exceeded("alice") {
		t.Fatal("alice should be exceeded")
	}
	if rl.Exceeded("bob") {
		t.Fatal("bob should not be exceeded")
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)

	rl.Record("alice")
	rl.Record("alice")
	rl.Record("alice")
	if !rl.Exceeded("alice") {
		t.Fatal("should be exceeded within window")
	}

	time.Sleep(60 * time.Millisecond)

	if rl.Exceeded("alice") {
		t.Fatal("should be reset after window expires")
	}

	rl.Record("alice")
	if rl.Exceeded("alice") {
		t.Fatal("single attempt after reset should not exceed")
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	rl := NewRateLimiter(1000, 10*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			identity := "worker"
			for j := 0; j < 100; j++ {
				rl.Record(identity)
				_ = rl.Exceeded(identity)
			}
		}(i)
	}
	wg.Wait()

	if !rl.Exceeded("worker") {
		t.Fatal("should be exceeded after 10000 concurrent records")
	}
}
