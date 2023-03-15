package redis_lock

import (
    "context"
    "github.com/Zzaniu/tool/distributed-lock"
    "github.com/go-basic/uuid"
    "github.com/go-redis/redis/v8"
    "golang.org/x/xerrors"
    "strconv"
    "time"
)

const (
    refresh = `if redis.call("get", KEYS[1]) == ARGV[1] then 
                return redis.call("pexpire", KEYS[1], ARGV[2]) 
            else 
                return 0 
            end`
    unlock = `if redis.call("GET", KEYS[1]) == ARGV[1] then
                return redis.call("DEL", KEYS[1])
            else
                return 0
            end`
)

var (
    unlockScript  = redis.NewScript(unlock)
    refreshScript = redis.NewScript(refresh)
)

type redisLock struct {
    // 因为集群的话，需要用红锁才能保证安全，所以这里写死 redis.Client
    client *redis.Client
    ctx    context.Context
    expire time.Duration
    closer chan struct{}
    uuid   string
    key    string
}

func NewRedisLock(client *redis.Client) distributed_lock.DistributedLock {
    return &redisLock{client: client, uuid: uuid.New(), ctx: context.Background()}
}

func NewRedisLockWithContext(ctx context.Context, client *redis.Client) distributed_lock.DistributedLock {
    return &redisLock{client: client, uuid: uuid.New(), ctx: ctx}
}

func (r *redisLock) Lock(key string, expire int, opts ...distributed_lock.Options) error {
    select {
    case <-r.ctx.Done():
        return distributed_lock.LockOccupied
    default:
    }
    opt := &distributed_lock.Option{Retry: distributed_lock.NoRetry()}
    for _, o := range opts {
        o(opt)
    }
    var ticker *time.Ticker
    for {
        ok, err := r.lock(key, expire)
        if err != nil {
            return err
        } else if ok {
            r.key = key
            r.expire = time.Duration(expire) * time.Second
            r.closer = make(chan struct{})
            if opt.Lease {
                // 续租
                go func() { _ = r.refresh() }()
            }
            return nil
        }

        backoff := opt.Retry.NextBackoff()
        if backoff < 1 {
            return distributed_lock.LockOccupied
        }
        if ticker == nil {
            ticker = time.NewTicker(backoff)
            defer ticker.Stop()
        } else {
            ticker.Reset(backoff)
        }

        select {
        case <-r.ctx.Done():
            return distributed_lock.LockOccupied
        case <-ticker.C:
        }
    }
}

func (r *redisLock) lock(key string, expire int) (bool, error) {
    return r.client.SetNX(r.ctx, key, r.uuid, time.Duration(expire)*time.Second).Result()
}

func (r *redisLock) refresh() error {
    ticker := time.NewTicker(r.expire / 2)
    for {
        select {
        case <-r.ctx.Done():
            return nil
        case <-r.closer:
            return nil
        case <-ticker.C:
            select {
            case <-r.closer:
                return nil
            default:
            }
            ttlVal := strconv.FormatInt(int64(r.expire/time.Millisecond), 10)
            status, err := refreshScript.Run(r.ctx, r.client, []string{r.key}, r.uuid, ttlVal).Result()
            if err != nil {
                return err
            } else if status != int64(1) {
                return distributed_lock.LockOccupied
            }
        }
    }
}

func (r *redisLock) UnLock() error {
    if len(r.key) == 0 {
        panic("it is currently unlocked")
    }
    close(r.closer)
    res, err := unlockScript.Run(r.ctx, r.client, []string{r.key}, r.uuid).Result()
    if err != nil {
        return xerrors.Errorf("%w", err)
    }
    if i, ok := res.(int64); !ok || i != 1 {
        return distributed_lock.LockTimeout
    }
    return nil
}
