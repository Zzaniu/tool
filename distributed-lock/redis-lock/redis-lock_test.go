package redis_lock

import (
    "context"
    "errors"
    "fmt"
    "io"
    "net"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    distributed_lock "github.com/Zzaniu/tool/distributed-lock"
    "github.com/go-redis/redis/v8"
)

type lockEntry struct {
    owner   string
    expires time.Time
}

// hook 模拟脚本的原子执行结果，并禁止建立真实 Redis 连接。
// redis-lock_integration_test.go 在显式启用的本地 Redis 上验证 Lua 脚本本身。
type lockStore struct {
    mu      sync.Mutex
    entries map[string]lockEntry
    calls   map[string]int
    before  func(context.Context, string, *redis.Cmd) error
    after   func(context.Context, string, *redis.Cmd) error
}

func newLockStore(t *testing.T) (*lockStore, *redis.Client) {
    t.Helper()
    store := &lockStore{entries: make(map[string]lockEntry), calls: make(map[string]int)}
    client := redis.NewClient(&redis.Options{
        MaxRetries: -1,
        Dialer: func(context.Context, string, string) (net.Conn, error) {
            return nil, errors.New("unit tests must not connect to Redis")
        },
    })
    client.AddHook(store)
    t.Cleanup(func() { _ = client.Close() })
    return store, client
}

func scriptKind(cmd *redis.Cmd) string {
    hash := fmt.Sprint(cmd.Args()[1])
    if cmd.Name() == "eval" {
        hash = redis.NewScript(hash).Hash()
    }
    switch hash {
    case acquireScript.Hash():
        return "acquire"
    case refreshScript.Hash():
        return "refresh"
    case unlockScript.Hash():
        return "unlock"
    }
    return "unknown"
}

func (s *lockStore) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
    return ctx, errors.New("intercepted by lock test")
}

func (s *lockStore) AfterProcess(ctx context.Context, command redis.Cmder) error {
    cmd, ok := command.(*redis.Cmd)
    if !ok {
        command.SetErr(fmt.Errorf("unexpected command: %s", command.Name()))
        return nil
    }
    cmd.SetErr(nil)
    if ctx.Err() != nil {
        cmd.SetErr(ctx.Err())
        return nil
    }
    kind := scriptKind(cmd)
    s.mu.Lock()
    s.calls[kind]++
    s.mu.Unlock()
    if s.before != nil {
        if err := s.before(ctx, kind, cmd); err != nil {
            cmd.SetErr(err)
            return nil
        }
    }
    s.execute(cmd)
    if s.after != nil {
        if err := s.after(ctx, kind, cmd); err != nil {
            cmd.SetErr(err)
        }
    }
    return nil
}

func (s *lockStore) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
    return ctx, errors.New("pipelines are not supported by lock tests")
}

func (s *lockStore) AfterProcessPipeline(context.Context, []redis.Cmder) error {
    return errors.New("pipelines are not supported by lock tests")
}

func (s *lockStore) execute(cmd *redis.Cmd) {
    s.mu.Lock()
    defer s.mu.Unlock()
    key, owner := fmt.Sprint(cmd.Args()[3]), fmt.Sprint(cmd.Args()[4])
    entry, exists := s.entries[key]
    if exists && !time.Now().Before(entry.expires) {
        delete(s.entries, key)
        exists = false
    }
    status := int64(0)
    switch scriptKind(cmd) {
    case "acquire", "refresh":
        if (scriptKind(cmd) == "acquire" && !exists) || (exists && entry.owner == owner) {
            ttl := time.Duration(cmd.Args()[5].(int64)) * time.Millisecond
            s.entries[key] = lockEntry{owner, time.Now().Add(ttl)}
            status = 1
        }
    case "unlock":
        if !exists {
            status = 2
        } else if entry.owner == owner {
            delete(s.entries, key)
            status = 1
        }
    default:
        cmd.SetErr(errors.New("unknown lock script"))
    }
    cmd.SetVal(status)
}

func (s *lockStore) put(key, owner string, ttl time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.entries[key] = lockEntry{owner, time.Now().Add(ttl)}
}

func (s *lockStore) read(key string) (lockEntry, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    entry, ok := s.entries[key]
    return entry, ok && time.Now().Before(entry.expires)
}

func (s *lockStore) count(kind string) int {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.calls[kind]
}

func awaitSignal(t *testing.T, signal <-chan struct{}) {
    t.Helper()
    select {
    case <-signal:
    case <-time.After(3 * time.Second):
        t.Fatal("timed out waiting for lock operation")
    }
}

func mustLock(t *testing.T, lock distributed_lock.DistributedLock, opts ...distributed_lock.Options) {
    t.Helper()
    if err := lock.Lock("test-lock", 1, opts...); err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = lock.UnLock() })
}

func TestLockValidation(t *testing.T) {
    _, client := newLockStore(t)
    for _, tc := range []struct {
        name   string
        client *redis.Client
        key    string
        ttl    int
    }{
        {"nil client", nil, "key", 1},
        {"empty key", client, "", 1},
        {"zero TTL", client, "key", 0},
        {"negative TTL", client, "key", -1},
    } {
        t.Run(tc.name, func(t *testing.T) {
            lock := NewRedisLock(tc.client)
            if err := lock.Lock(tc.key, tc.ttl, distributed_lock.WithLease()); err == nil {
                t.Fatal("invalid lock was acquired")
            }
        })
    }
    lock := NewManagedRedisLock(nil, client)
    if err := lock.UnLock(); !errors.Is(err, distributed_lock.LockNotHeld) {
        t.Fatalf("unlock before acquisition: %v", err)
    }
    mustLock(t, lock, nil, distributed_lock.WithRetry(nil), distributed_lock.WithRetryFactory(func() distributed_lock.RetryStrategy { return nil }))
    if err := lock.UnLockContext(nil); err == nil {
        t.Fatal("nil cleanup context accepted")
    }
}

func TestLockContentionAndLifecycle(t *testing.T) {
    store, client := newLockStore(t)
    first := NewManagedRedisLock(context.Background(), client)
    mustLock(t, first)
    second := NewRedisLock(client)
    if err := second.Lock("test-lock", 1); !errors.Is(err, distributed_lock.LockOccupied) {
        t.Fatalf("contention: %v", err)
    }
    if err := first.Lock("other-key", 1); !errors.Is(err, distributed_lock.LockInUse) {
        t.Fatalf("held instance reused: %v", err)
    }
    if err := first.UnLock(); err != nil {
        t.Fatal(err)
    }
    awaitSignal(t, first.Done())
    if first.Err() != nil {
        t.Fatalf("normal release Err = %v", first.Err())
    }
    mustLock(t, second)
    successor, _ := store.read("test-lock")
    if err := first.UnLock(); err != nil {
        t.Fatal(err)
    }
    if err := first.Lock("test-lock", 1); !errors.Is(err, distributed_lock.LockInUse) {
        t.Fatalf("released instance reused: %v", err)
    }
    if entry, ok := store.read("test-lock"); !ok || entry.owner != successor.owner {
        t.Fatal("old unlock changed the successor's lock")
    }
}

func TestConcurrentAcquisitionOnOneInstance(t *testing.T) {
    store, client := newLockStore(t)
    started, release := make(chan struct{}), make(chan struct{})
    store.before = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
        if kind == "acquire" {
            close(started)
            <-release
        }
        return nil
    }
    lock := NewRedisLock(client)
    result := make(chan error, 1)
    go func() { result <- lock.Lock("test-lock", 1) }()
    awaitSignal(t, started)
    err := lock.Lock("other-key", 1)
    unlockErr := lock.UnLock()
    close(release)
    if got := <-result; got != nil {
        t.Fatal(got)
    }
    defer lock.UnLock()
    if !errors.Is(err, distributed_lock.LockInUse) || !errors.Is(unlockErr, distributed_lock.LockInUse) {
        t.Fatalf("concurrent operations: Lock=%v, UnLock=%v", err, unlockErr)
    }
    if store.count("acquire") != 1 {
        t.Fatal("concurrent Lock reached Redis")
    }
}

func TestLostAcquisitionResponse(t *testing.T) {
    for _, mode := range []string{"client replay", "final error", "canceled response", "cleanup error"} {
        t.Run(mode, func(t *testing.T) {
            store, client := newLockStore(t)
            ctx, cancel := context.WithCancel(context.Background())
            defer cancel()
            cleanupErr := errors.New("cleanup unavailable")
            store.before = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
                if kind == "unlock" && mode == "cleanup error" {
                    return cleanupErr
                }
                return nil
            }
            store.after = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
                if kind != "acquire" {
                    return nil
                }
                if mode == "client replay" {
                    store.execute(cmd)
                    return nil
                }
                if mode == "canceled response" {
                    cancel()
                    return nil
                }
                return io.ErrUnexpectedEOF
            }
            lock := NewRedisLockWithContext(ctx, client)
            err := lock.Lock("test-lock", 1)
            switch mode {
            case "client replay":
                if err != nil {
                    t.Fatalf("replayed acquisition: %v", err)
                }
                if err := lock.UnLock(); err != nil {
                    t.Fatal(err)
                }
            case "canceled response":
                if !errors.Is(err, context.Canceled) {
                    t.Fatalf("canceled acquisition: %v", err)
                }
            case "cleanup error":
                if !errors.Is(err, io.ErrUnexpectedEOF) || !errors.Is(err, cleanupErr) {
                    t.Fatalf("lost cleanup error: %v", err)
                }
                return
            default:
                if !errors.Is(err, io.ErrUnexpectedEOF) {
                    t.Fatalf("lost response error: %v", err)
                }
            }
            if _, exists := store.read("test-lock"); exists {
                t.Fatal("acquisition left its key behind")
            }
        })
    }
}

type signaledRetry struct{ started chan struct{} }

func (r signaledRetry) NextBackoff() time.Duration {
    close(r.started)
    return time.Hour
}

func TestCancelWhileWaitingForLock(t *testing.T) {
    store, client := newLockStore(t)
    store.put("test-lock", "other-owner", time.Minute)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    waiting := make(chan struct{})
    result := make(chan error, 1)
    go func() {
        result <- NewRedisLockWithContext(ctx, client).Lock("test-lock", 1, distributed_lock.WithRetry(signaledRetry{waiting}))
    }()
    awaitSignal(t, waiting)
    cancel()
    if err := <-result; !errors.Is(err, context.Canceled) {
        t.Fatalf("wait cancellation = %v", err)
    }
    if entry, exists := store.read("test-lock"); !exists || entry.owner != "other-owner" {
        t.Fatal("cancellation deleted another owner's key")
    }
}

func TestUnlockAfterCancellation(t *testing.T) {
    store, client := newLockStore(t)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    lock := NewManagedRedisLock(ctx, client)
    mustLock(t, lock, distributed_lock.WithLease())
    cancel()
    awaitSignal(t, lock.Done())
    if !errors.Is(lock.Err(), context.Canceled) {
        t.Fatalf("cancellation Err = %v", lock.Err())
    }
    if err := lock.UnLock(); err != nil {
        t.Fatal(err)
    }
    if _, exists := store.read("test-lock"); exists {
        t.Fatal("canceled request left the lock held")
    }
}

func TestUnlockRetryAndConcurrentCalls(t *testing.T) {
    store, client := newLockStore(t)
    var unlocks atomic.Int32
    store.before = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
        if kind == "unlock" && unlocks.Add(1) == 1 {
            return io.ErrUnexpectedEOF
        }
        return nil
    }
    lock := NewRedisLock(client)
    mustLock(t, lock, distributed_lock.WithLease())
    if err := lock.UnLock(); !errors.Is(err, io.ErrUnexpectedEOF) {
        t.Fatalf("first unlock = %v", err)
    }
    results := make(chan error, 16)
    for i := 0; i < cap(results); i++ {
        go func() { results <- lock.UnLock() }()
    }
    for i := 0; i < cap(results); i++ {
        if err := <-results; err != nil {
            t.Errorf("concurrent retry: %v", err)
        }
    }
    if store.count("unlock") != 2 {
        t.Fatalf("unlock calls = %d, want 2", store.count("unlock"))
    }
    if _, exists := store.read("test-lock"); exists {
        t.Fatal("unlock retry left the key")
    }
}

func TestUnlockReplayAndOwnerProtection(t *testing.T) {
    for _, changedOwner := range []bool{false, true} {
        t.Run(fmt.Sprint(changedOwner), func(t *testing.T) {
            store, client := newLockStore(t)
            store.after = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
                if kind == "unlock" && !changedOwner {
                    store.execute(cmd)
                }
                return nil
            }
            lock := NewRedisLock(client)
            mustLock(t, lock)
            if changedOwner {
                store.put("test-lock", "successor", time.Minute)
            }
            err := lock.UnLock()
            if changedOwner {
                if !errors.Is(err, distributed_lock.LockTimeout) {
                    t.Fatalf("foreign owner unlock = %v", err)
                }
                if entry, ok := store.read("test-lock"); !ok || entry.owner != "successor" {
                    t.Fatal("deleted successor's lock")
                }
            } else if err != nil {
                t.Fatalf("replayed unlock = %v", err)
            }
        })
    }
}

func TestUnlockContextCanBeRetried(t *testing.T) {
    store, client := newLockStore(t)
    lock := NewManagedRedisLock(context.Background(), client)
    mustLock(t, lock)
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    if err := lock.UnLockContext(ctx); !errors.Is(err, context.Canceled) {
        t.Fatalf("canceled cleanup = %v", err)
    }
    if err := lock.UnLockContext(context.Background()); err != nil {
        t.Fatal(err)
    }
    if _, exists := store.read("test-lock"); exists {
        t.Fatal("cleanup retry left the key")
    }
}

func TestConcurrentUnlockHonorsContext(t *testing.T) {
    store, client := newLockStore(t)
    started, release := make(chan struct{}), make(chan struct{})
    store.before = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
        if kind == "unlock" {
            close(started)
            select {
            case <-release:
            case <-ctx.Done():
                return ctx.Err()
            }
        }
        return nil
    }
    lock := NewManagedRedisLock(context.Background(), client)
    mustLock(t, lock)
    first := make(chan error, 1)
    go func() { first <- lock.UnLock() }()
    awaitSignal(t, started)
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    err := lock.UnLockContext(ctx)
    close(release)
    if firstErr := <-first; firstErr != nil {
        t.Fatal(firstErr)
    }
    if !errors.Is(err, context.Canceled) {
        t.Fatalf("waiting unlock ignored cancellation: %v", err)
    }
}

func TestLeaseLossNotification(t *testing.T) {
    for _, mode := range []string{"renew errors", "owner changed", "fixed expiry", "transient error"} {
        t.Run(mode, func(t *testing.T) {
            t.Parallel()
            store, client := newLockStore(t)
            var attempts atomic.Int32
            renewed := make(chan struct{}, 4)
            store.before = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
                if kind == "refresh" {
                    n := attempts.Add(1)
                    if mode == "renew errors" || (mode == "transient error" && n == 1) {
                        return io.ErrUnexpectedEOF
                    }
                }
                return nil
            }
            store.after = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
                if kind == "refresh" {
                    select {
                    case renewed <- struct{}{}:
                    default:
                    }
                }
                return nil
            }
            lock := NewManagedRedisLock(context.Background(), client)
            if mode == "fixed expiry" {
                mustLock(t, lock)
            } else {
                mustLock(t, lock, distributed_lock.WithLease())
            }
            if mode == "owner changed" {
                store.put("test-lock", "successor", time.Minute)
            }
            if mode == "transient error" {
                awaitSignal(t, renewed)
                awaitSignal(t, renewed)
                if lock.Err() != nil {
                    t.Fatalf("renewal did not recover: %v", lock.Err())
                }
                if entry, ok := store.read("test-lock"); !ok || time.Until(entry.expires) < 500*time.Millisecond {
                    t.Fatal("renewal did not extend expiration")
                }
                return
            }
            awaitSignal(t, lock.Done())
            if !errors.Is(lock.Err(), distributed_lock.LockTimeout) {
                t.Fatalf("lease loss Err = %v", lock.Err())
            }
            if mode == "renew errors" && (!errors.Is(lock.Err(), io.ErrUnexpectedEOF) || attempts.Load() != maxRenewErrors) {
                t.Fatalf("renew errors: attempts=%d, err=%v", attempts.Load(), lock.Err())
            }
        })
    }
}

func TestLeaseRenewalResponseAfterDeadline(t *testing.T) {
    for _, tc := range []struct {
        name          string
        responseDelay time.Duration
        responseErr   error
        wantRenewal   bool
    }{
        {"success", 0, nil, true},
        {"timeout", 0, context.DeadlineExceeded, false},
        {"expired success", 1100 * time.Millisecond, nil, false},
    } {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            store, client := newLockStore(t)
            ctx, cancel := context.WithCancel(context.Background())
            defer cancel()
            lock := NewManagedRedisLock(ctx, client).(*redisLock)
            ttl := time.Second
            deadline := time.Now().Add(500 * time.Millisecond)
            store.put("test-lock", "owner", ttl)
            renewed := make(chan struct{})
            store.after = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
                if kind != "refresh" {
                    return nil
                }
                switch store.count("refresh") {
                case 1:
                    delay := time.Until(deadline.Add(50 * time.Millisecond))
                    if tc.responseDelay > 0 {
                        delay = tc.responseDelay
                    }
                    // 模拟收到结果后客户端处理延迟，期间请求的 context 可先到期。
                    time.Sleep(delay)
                    return tc.responseErr
                case 2:
                    close(renewed)
                }
                return nil
            }
            workerDone := make(chan struct{})
            go func() {
                defer close(workerDone)
                lock.watch(ctx, "test-lock", "owner", ttl, deadline, true)
            }()
            t.Cleanup(func() {
                cancel()
                awaitSignal(t, workerDone)
            })
            if tc.wantRenewal {
                select {
                case <-renewed:
                case <-lock.Done():
                    t.Fatalf("successful renewal stopped the watcher: %v", lock.Err())
                case <-time.After(3 * time.Second):
                    t.Fatal("timed out waiting for the next renewal")
                }
                if err := lock.Err(); err != nil {
                    t.Fatalf("renewal Err = %v", err)
                }
                if entry, ok := store.read("test-lock"); !ok || entry.owner != "owner" {
                    t.Fatal("renewal did not preserve ownership")
                }
                return
            }
            awaitSignal(t, lock.Done())
            awaitSignal(t, workerDone)
            if !errors.Is(lock.Err(), distributed_lock.LockTimeout) {
                t.Fatalf("lease loss Err = %v", lock.Err())
            }
            if tc.responseErr != nil && !errors.Is(lock.Err(), tc.responseErr) {
                t.Fatalf("lost renewal error: %v", lock.Err())
            }
            if got := store.count("refresh"); got != 1 {
                t.Fatalf("refresh calls = %d, want 1", got)
            }
        })
    }
}

func TestExpiredAcquisitionIsRejected(t *testing.T) {
    t.Parallel()
    store, client := newLockStore(t)
    store.after = func(ctx context.Context, kind string, cmd *redis.Cmd) error {
        if kind == "acquire" {
            time.Sleep(1100 * time.Millisecond)
        }
        return nil
    }
    lock := NewRedisLock(client)
    if err := lock.Lock("test-lock", 1); !errors.Is(err, distributed_lock.LockTimeout) {
        t.Fatalf("expired acquisition = %v", err)
    }
    if _, exists := store.read("test-lock"); exists {
        t.Fatal("expired acquisition left the key")
    }
}
