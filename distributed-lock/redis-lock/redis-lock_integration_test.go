package redis_lock

import (
    "context"
    "net"
    "os"
    "testing"
    "time"

    "github.com/go-basic/uuid"
    "github.com/go-redis/redis/v8"
)

// 将 REDIS_LOCK_TEST_ADDR 设置为独立的本地回环地址 Redis，以执行真实 Lua 脚本。
// 测试仅操作唯一的测试 key，不会清空数据库。
func TestRedisScriptsIntegration(t *testing.T) {
    addr := os.Getenv("REDIS_LOCK_TEST_ADDR")
    if addr == "" {
        t.Skip("REDIS_LOCK_TEST_ADDR is not set")
    }
    host, _, err := net.SplitHostPort(addr)
    if err != nil || !net.ParseIP(host).IsLoopback() {
        t.Fatal("REDIS_LOCK_TEST_ADDR must be a loopback IP and port")
    }
    client := redis.NewClient(&redis.Options{
        Addr: addr, MaxRetries: -1,
        DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
    })
    defer client.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    key := "tool:test:redis-lock:" + uuid.New()
    defer client.Del(context.Background(), key)
    check := func(script *redis.Script, want int64, args ...interface{}) {
        t.Helper()
        // 使用 EVAL 执行 Lua 脚本正文，不依赖 Redis 中已有的脚本缓存。
        got, err := script.Eval(ctx, client, []string{key}, args...).Int64()
        if err != nil || got != want {
            t.Fatalf("script result = %d, %v; want %d", got, err, want)
        }
    }
    check(acquireScript, 1, "first", 5000)
    check(acquireScript, 1, "first", 10000)
    if ttl := client.PTTL(ctx, key).Val(); ttl <= 5*time.Second || ttl > 10*time.Second {
        t.Fatalf("replayed acquisition TTL = %v", ttl)
    }
    check(acquireScript, 0, "second", 5000)
    check(refreshScript, 0, "second", 5000)
    check(unlockScript, 0, "second")
    check(refreshScript, 1, "first", 15000)
    check(unlockScript, 1, "first")
    check(unlockScript, 2, "first")
    check(refreshScript, 0, "first", 5000)
    check(acquireScript, 1, "second", 5000)
    check(unlockScript, 0, "first")
    check(unlockScript, 1, "second")
}
