package redis_lock

import (
    "context"
    "errors"
    "fmt"
    "sync"
    "time"

    distributed_lock "github.com/Zzaniu/tool/distributed-lock"
    "github.com/go-basic/uuid"
    "github.com/go-redis/redis/v8"
)

const (
    cleanupTimeout = 2 * time.Second
    maxRenewErrors = 3

    // 同一次加锁请求重试时续租，避免把自身的锁误判为被其他持有者占用。
    acquire = `local owner = redis.call("GET", KEYS[1])
               if not owner then
                   redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
                   return 1
               elseif owner == ARGV[1] then
                   return redis.call("PEXPIRE", KEYS[1], ARGV[2])
               end
               return 0`
    refresh = `if redis.call("GET", KEYS[1]) == ARGV[1] then
                   return redis.call("PEXPIRE", KEYS[1], ARGV[2])
               end
               return 0`
    // key 不存在时视为清理成功，兼容 DEL 成功但响应丢失后的重试。
    unlock = `local owner = redis.call("GET", KEYS[1])
              if not owner then
                  return 2
              elseif owner == ARGV[1] then
                  return redis.call("DEL", KEYS[1])
              end
              return 0`
)

var (
    acquireScript = redis.NewScript(acquire)
    unlockScript  = redis.NewScript(unlock)
    refreshScript = redis.NewScript(refresh)
)

type redisLock struct {
    client *redis.Client
    ctx    context.Context

    mu          sync.Mutex
    releaseGate chan struct{}
    acquiring   bool
    used        bool
    released    bool
    releaseErr  error
    key         string
    owner       string
    stop        context.CancelFunc
    workerDone  chan struct{}
    done        chan struct{}
    finished    bool
    err         error
}

var _ distributed_lock.ManagedLock = (*redisLock)(nil)

func NewRedisLock(client *redis.Client) distributed_lock.DistributedLock {
    return NewManagedRedisLock(context.Background(), client)
}

func NewRedisLockWithContext(ctx context.Context, client *redis.Client) distributed_lock.DistributedLock {
    return NewManagedRedisLock(ctx, client)
}

// NewManagedRedisLock 支持指定清理 context，并提供失去持锁保证的通知。
// 每个实例只能成功加锁一次，下一次加锁需创建新实例。
func NewManagedRedisLock(ctx context.Context, client *redis.Client) distributed_lock.ManagedLock {
    if ctx == nil {
        ctx = context.Background()
    }
    return &redisLock{client: client, ctx: ctx, done: make(chan struct{}), releaseGate: make(chan struct{}, 1)}
}

func (r *redisLock) Lock(key string, expire int, opts ...distributed_lock.Options) error {
    if r.client == nil || key == "" || expire <= 0 || int64(expire) > int64((1<<63-1)/time.Second) {
        return fmt.Errorf("invalid Redis lock client, key or expiration")
    }
    if err := r.ctx.Err(); err != nil {
        return err
    }
    opt := &distributed_lock.Option{Retry: distributed_lock.NoRetry()}
    for _, o := range opts {
        if o != nil {
            o(opt)
        }
    }
    if opt.Retry == nil {
        opt.Retry = distributed_lock.NoRetry()
    }
    r.mu.Lock()
    if r.acquiring || r.used {
        r.mu.Unlock()
        return distributed_lock.LockInUse
    }
    r.acquiring = true
    r.mu.Unlock()
    defer func() {
        r.mu.Lock()
        r.acquiring = false
        r.mu.Unlock()
    }()

    owner := uuid.New()
    ttl := time.Duration(expire) * time.Second
    deadline, err := r.acquire(key, owner, ttl, opt.Retry)
    if err != nil {
        cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), cleanupTimeout)
        defer cancel()
        if _, cleanupErr := r.release(cleanupCtx, key, owner); cleanupErr != nil {
            return errors.Join(err, fmt.Errorf("cleanup failed acquisition: %w", cleanupErr))
        }
        return err
    }

    workerCtx, stop := context.WithCancel(r.ctx)
    workerDone := make(chan struct{})
    r.mu.Lock()
    r.used = true
    r.key, r.owner = key, owner
    r.stop, r.workerDone = stop, workerDone
    r.mu.Unlock()
    go func() {
        defer close(workerDone)
        r.watch(workerCtx, key, owner, ttl, deadline, opt.Lease)
    }()
    return nil
}

func (r *redisLock) acquire(key, owner string, ttl time.Duration, retry distributed_lock.RetryStrategy) (time.Time, error) {
    for {
        if err := r.ctx.Err(); err != nil {
            return time.Time{}, err
        }
        started := time.Now()
        status, err := acquireScript.Run(r.ctx, r.client, []string{key}, owner, ttl.Milliseconds()).Int64()
        if err != nil {
            return time.Time{}, fmt.Errorf("acquire Redis lock: %w", err)
        }
        if err := r.ctx.Err(); err != nil {
            return time.Time{}, err
        }
        if status == 1 {
            deadline := started.Add(ttl)
            if !time.Now().Before(deadline) {
                return time.Time{}, distributed_lock.LockTimeout
            }
            return deadline, nil
        }
        if status != 0 {
            return time.Time{}, fmt.Errorf("unexpected Redis lock response: %d", status)
        }
        backoff := retry.NextBackoff()
        if backoff <= 0 {
            return time.Time{}, distributed_lock.LockOccupied
        }
        if err := wait(r.ctx, backoff); err != nil {
            return time.Time{}, err
        }
    }
}

func wait(ctx context.Context, delay time.Duration) error {
    timer := time.NewTimer(delay)
    defer timer.Stop()
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        return ctx.Err()
    }
}

func (r *redisLock) watch(ctx context.Context, key, owner string, ttl time.Duration, deadline time.Time, lease bool) {
    delay := time.Until(deadline)
    if lease {
        delay /= 2
    }
    failures := 0
    for {
        if err := wait(ctx, delay); err != nil {
            r.finish(err)
            return
        }
        if !lease || !time.Now().Before(deadline) {
            r.finish(distributed_lock.LockTimeout)
            return
        }
        started := time.Now()
        attemptDeadline := minTime(deadline, started.Add(cleanupTimeout))
        attemptCtx, cancel := context.WithDeadline(ctx, attemptDeadline)
        status, err := refreshScript.Run(attemptCtx, r.client, []string{key}, owner, ttl.Milliseconds()).Int64()
        cancel()
        if ctx.Err() != nil {
            r.finish(ctx.Err())
            return
        }
        if err == nil && status == 1 {
            // 成功响应确认了新租期，客户端处理延迟可能已跨过旧截止时间。
            deadline = started.Add(ttl)
        }
        if !time.Now().Before(deadline) {
            r.finish(errors.Join(distributed_lock.LockTimeout, err))
            return
        }
        if err != nil {
            failures++
            if failures >= maxRenewErrors {
                r.finish(errors.Join(distributed_lock.LockTimeout, fmt.Errorf("renew Redis lock: %w", err)))
                return
            }
            delay = min(50*time.Millisecond, time.Until(deadline)/2)
            continue
        }
        if status != 1 {
            r.finish(distributed_lock.LockTimeout)
            return
        }
        failures = 0
        delay = time.Until(deadline) / 2
    }
}

func minTime(a, b time.Time) time.Time {
    if a.Before(b) {
        return a
    }
    return b
}

func (r *redisLock) Done() <-chan struct{} {
    return r.done
}

func (r *redisLock) Err() error {
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.err
}

func (r *redisLock) finish(err error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if !r.finished {
        r.finished, r.err = true, err
        close(r.done)
    }
}

// UnLock 使用独立的清理 context，加锁时的 context 取消后仍可释放锁。
func (r *redisLock) UnLock() error {
    ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), cleanupTimeout)
    defer cancel()
    return r.UnLockContext(ctx)
}

// UnLockContext 在 Redis 操作失败后可再次调用，成功清理后重复调用不会产生副作用。
// 清理时校验持有者标识，不会删除其他持有者的锁。
// 尚未持锁且正在加锁时返回 LockInUse，不等待或取消在途加锁。
// Done 关闭不表示 Redis 清理成功；清理结果由本方法返回，不会改写 Err。
func (r *redisLock) UnLockContext(ctx context.Context) error {
    if ctx == nil {
        return fmt.Errorf("nil Redis unlock context")
    }
    select {
    case r.releaseGate <- struct{}{}:
        defer func() { <-r.releaseGate }()
    case <-ctx.Done():
        return ctx.Err()
    }
    r.mu.Lock()
    if !r.used {
        acquiring := r.acquiring
        r.mu.Unlock()
        if acquiring {
            return distributed_lock.LockInUse
        }
        return distributed_lock.LockNotHeld
    }
    if r.released {
        err := r.releaseErr
        r.mu.Unlock()
        return err
    }
    key, owner, stop, workerDone := r.key, r.owner, r.stop, r.workerDone
    r.mu.Unlock()
    r.finish(nil)
    stop()
    select {
    case <-workerDone:
    case <-ctx.Done():
        return ctx.Err()
    }
    status, err := r.release(ctx, key, owner)
    if err != nil {
        return fmt.Errorf("release Redis lock: %w", err)
    }
    if status == 0 {
        err = distributed_lock.LockTimeout
    }
    r.mu.Lock()
    r.released, r.releaseErr = true, err
    r.mu.Unlock()
    return err
}

func (r *redisLock) release(ctx context.Context, key, owner string) (int64, error) {
    status, err := unlockScript.Run(ctx, r.client, []string{key}, owner).Int64()
    if err == nil && (status < 0 || status > 2) {
        err = fmt.Errorf("unexpected Redis unlock response: %d", status)
    }
    return status, err
}
