package redis_lock

import (
    "github.com/Zzaniu/tool/distributed-lock"
    "github.com/go-basic/uuid"
    "github.com/go-redis/redis"
    "golang.org/x/xerrors"
    "sync/atomic"
    "time"
)

const (
    unlock = `if redis.call("GET", KEYS[1]) == ARGV[1] then
                return redis.call("DEL", KEYS[1])
            else
                return 0
            end`
)

var (
    unlockScript    = redis.NewScript(unlock)
    unlockScriptSha atomic.Value
)

type redisLock struct {
    // 因为集群的话，需要用红锁才能保证安全，所以这里写死 redis.Client
    client *redis.Client
    uuid   string
    key    string
}

func NewRedisLock(client *redis.Client) distributed_lock.DistributedLock {
    return &redisLock{client: client, uuid: uuid.New()}
}

func (r *redisLock) Lock(key string, expire int) error {
    result, err := r.client.SetNX(key, r.uuid, time.Duration(expire)*time.Second).Result()
    if err != nil {
        return xerrors.Errorf("%w", err)
    }
    if !result {
        return distributed_lock.LockOccupied
    }
    r.key = key
    return nil
}

func (r *redisLock) UnLock() error {
    if len(r.key) == 0 {
        panic("it is currently unlocked")
    }
    sha, ok := unlockScriptSha.Load().(string)
    if !ok {
        var err error
        sha, err = unlockScript.Load(r.client).Result()
        if err != nil {
            return xerrors.Errorf("Store Load error: %w", err)
        }
        unlockScriptSha.Store(sha)
    }
    reti, err := r.client.EvalSha(sha, []string{r.key}, r.uuid).Result()
    if err != nil {
        return xerrors.Errorf("%w", err)
    }
    ret := reti.(int64)
    if ret != 1 {
        return distributed_lock.LockTimeout
    }
    return nil
}
