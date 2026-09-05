# Redis 分布式锁

`NewRedisLock` 和 `NewRedisLockWithContext` 保留原有的 `DistributedLock` 接口。
加锁和等待使用构造时传入的 context。`UnLock()` 使用独立的清理 context，
超时时间为 2 秒，因此业务请求取消后仍然可以释放锁。

同一个锁实例允许在加锁失败后重试，但只能成功加锁一次。下一次执行需要锁保护的业务时，
应创建新实例。对正在加锁或已经成功使用过的实例并发、重复调用 `Lock`，会返回 `LockInUse`。
尚未成功加锁时调用 `UnLock` 或 `UnLockContext`：若加锁正在进行，返回 `LockInUse`；
否则返回 `LockNotHeld`。释放方法不会等待或取消在途加锁，要中止加锁需取消构造时传入的 context。
成功释放后再次调用不会重复操作 Redis。
如果释放锁时 Redis 请求失败，可以再次尝试释放。如果锁已属于其他持有者，
会返回 `LockTimeout` 并保留对方的锁；如果 key 已不存在，则视为清理成功。

## 持锁状态监控

`NewManagedRedisLock(ctx, client)` 在原有方法之外，提供 `Done()`、`Err()`
和 `UnLockContext(cleanupCtx)`，用于监控持锁状态及指定释放锁时使用的 context。

```go
lock := redis_lock.NewManagedRedisLock(ctx, client)
if err := lock.Lock(key, 10, distributed_lock.WithLease()); err != nil {
    return err
}
defer func() {
    if err := lock.UnLock(); err != nil {
        log.Printf("释放锁失败: %v", err)
    }
}()

workCtx, cancel := context.WithCancel(ctx)
defer cancel()
go func() {
    select {
    case <-lock.Done():
        cancel()
    case <-workCtx.Done():
    }
}()

err := doWork(workCtx)
if lock.Err() != nil {
    return lock.Err()
}
return err
```

开始释放锁、构造时传入的 context 被取消、固定租期到期、续租时发现持有者变化，
或无法确认续租成功时，`Done()` 返回的 channel 会关闭。
`Done()` 通知的是本地持锁保证已结束，不表示 Redis key 已删除。
`Err()` 保留首次结束持锁监控的原因；若主动释放先触发结束，则为 `nil`，
即使后续 Redis 清理失败也不会改变。释放结果以 `UnLock` 或 `UnLockContext` 的返回值为准。
续租连续失败时最多尝试 3 次，每次请求的超时期限不晚于当时已确认的租期截止时间。
确认续租成功后，按本次请求开始时间加 TTL 计算新的保守截止时间；
即使成功结果的处理跨过旧截止时间，只要新租期仍有效，就继续续租。
续租失败的错误会同时保留 `LockTimeout` 和底层 Redis 错误，可通过 `errors.Is` 判断。

取消需要业务配合：调用方必须在 `Done()` 关闭后停止受锁保护的操作。
锁库不会强行终止业务 goroutine，也不提供用于阻止旧持有者继续写入的 fencing token（隔离令牌）。
如果 Redis 不可访问，加锁或释放失败可能导致 key 保留到 TTL 到期。
锁库会尝试校验持有者后清理，并返回清理失败的错误。

## 重试策略

`WithRetry(RetreatRetry(n))` 会为每次 `Lock` 创建独立的内置重试状态。
`WithRetry(nil)` 表示不重试。对于带有可变状态的自定义策略，应使用
`WithRetryFactory(func() distributed_lock.RetryStrategy { ... })`，
工厂函数必须在每次调用时返回一个新的策略实例。
context 取消时返回实际的取消或超时错误；锁竞争重试耗尽时返回 `LockOccupied`。

## 验证

```sh
go test ./distributed-lock/... -count=10
go test -race ./distributed-lock/...
```

单元测试使用 Redis hook 模拟命令结果，禁止连接真实 Redis。
如需在独立的本地 Redis 上验证 Lua 脚本，设置环境变量
`REDIS_LOCK_TEST_ADDR=127.0.0.1:6379`，然后执行：

```sh
go test ./distributed-lock/redis-lock -run TestRedisScriptsIntegration -count=1
```

集成测试仅使用唯一的测试 key，不会清空数据库。
