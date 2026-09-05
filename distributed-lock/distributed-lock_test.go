package distributed_lock

import (
    "sync"
    "testing"
    "time"
)

func TestRetryOptionHasIndependentState(t *testing.T) {
    option := WithRetry(RetreatRetry(2))
    var wg sync.WaitGroup
    for i := 0; i < 32; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            opt := &Option{}
            option(opt)
            for _, want := range []time.Duration{time.Millisecond, 4 * time.Millisecond, 0} {
                if got := opt.Retry.NextBackoff(); got != want {
                    t.Errorf("backoff = %v, want %v", got, want)
                }
            }
        }()
    }
    wg.Wait()
}

func TestRetryConcurrentAccess(t *testing.T) {
    retry := RetreatRetry(16)
    results := make(chan time.Duration, 32)
    for i := 0; i < cap(results); i++ {
        go func() { results <- retry.NextBackoff() }()
    }
    seen := make(map[time.Duration]bool)
    for i := 0; i < cap(results); i++ {
        if duration := <-results; duration != 0 {
            if seen[duration] {
                t.Errorf("duplicate backoff: %v", duration)
            }
            seen[duration] = true
        }
    }
    if len(seen) != 16 {
        t.Fatalf("got %d attempts, want 16", len(seen))
    }
}

func TestRetryFactoryAndDefaults(t *testing.T) {
    option := WithRetryFactory(func() RetryStrategy { return RetreatRetry(1) })
    for i := 0; i < 2; i++ {
        opt := &Option{}
        option(opt)
        if got := opt.Retry.NextBackoff(); got != time.Millisecond {
            t.Fatalf("factory backoff = %v", got)
        }
    }
    opt := &Option{}
    WithRetry(nil)(opt)
    if got := opt.Retry.NextBackoff(); got != 0 {
        t.Fatalf("nil retry backoff = %v", got)
    }
    for _, max := range []int{-1, 0} {
        if got := RetreatRetry(max).NextBackoff(); got != 0 {
            t.Fatalf("max=%d backoff=%v", max, got)
        }
    }
    large := &retreatRetry{max: 10000000, num: 9999999}
    if got := large.NextBackoff(); got != time.Duration(1<<63-1) {
        t.Fatalf("overflow backoff = %v", got)
    }
}
