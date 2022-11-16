package window

import (
	"sync"
	"time"
)

// RollingPolicy 滑块
type (
	RollingPolicy struct {
		mu             sync.RWMutex
		window         Window
		size           int
		offset         int
		bucketDuration time.Duration
		lastAppendTime time.Time
	}
)

// Add 桶数据增 这里会触发滑动
func (r *RollingPolicy) Add(val uint64) {
	// 这里是写入数据，要加写锁
	r.mu.Lock()
	defer r.mu.Unlock()
	rawMoveBucketCount := r.timespan()
	if rawMoveBucketCount > 0 {
		moveBucketCount := rawMoveBucketCount
		if moveBucketCount > r.size {
			moveBucketCount = r.size
		}
		// 大于0的话，说明有移动，那么需要清理
		r.window.ResetBuckets((r.offset+1)%r.size, moveBucketCount)
		r.offset = (r.offset + moveBucketCount) % r.size
		r.lastAppendTime = r.lastAppendTime.Add(r.bucketDuration * time.Duration(rawMoveBucketCount))
	}
	r.window.Add(r.offset, val)
}

// timespan 过了几个bucket
func (r *RollingPolicy) timespan() int {
	// 这里是直接向下取整的
	span := int(time.Since(r.lastAppendTime) / r.bucketDuration)
	if span > -1 {
		return span
	}
	return r.size
}

// Reduce 统计所有的采集完整的桶, 当前这个桶和跳过的桶不计算
func (r *RollingPolicy) Reduce(fn func(*Bucket)) {
	// Reduce只是读取数据，加个只读锁就好了
	r.mu.RLock()
	defer r.mu.RUnlock()
	moveBucketCount := r.timespan()
	var statCount int
	if moveBucketCount == 0 {
		statCount = r.size - 1 // 没有移动的话，当前这个不要，因为当前这个还在采集中啊，还没采集满
	} else {
		// 跳过那些没有统计的, 和当前这一个
		statCount = r.size - moveBucketCount // 如果 movBucketCount == r.size 说明已经有超过一个window周期的时间未有采集数据
	}
	if statCount > 0 {
		offset := (r.offset + moveBucketCount + 1) % r.size
		r.window.Reduce(offset, statCount, fn)
	}
}

// Summary 返回有效bucket数量之和
func (r *RollingPolicy) Summary() (accept uint64, count uint64) {
	r.Reduce(func(bucket *Bucket) {
		accept += bucket.Sum
		count += bucket.Count
	})
	return
}

func WithBucket(bucketSize int) Option {
	return func(o *options) {
		o.Bucket = bucketSize
	}
}

func WithBucketDuration(bucketDuration time.Duration) Option {
	return func(o *options) {
		o.BucketDuration = bucketDuration
	}
}

func NewRollingPolicy(opts ...Option) *RollingPolicy {
	opt := options{
		BucketDuration: time.Millisecond * 100, // 一个bucket, 100ms
		Bucket:         10,                     // 10个桶
	}

	for _, o := range opts {
		o(&opt)
	}

	return &RollingPolicy{
		window:         NewWindow(opt.Bucket),
		size:           opt.Bucket,
		bucketDuration: opt.BucketDuration,
		lastAppendTime: time.Now(),
	}
}
