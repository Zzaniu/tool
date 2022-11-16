package window

type Breaker interface {
    Allow() (bool, error)
}

type window struct {
    buckets []*Bucket
    size    int
}

// Add 对应桶数量加
func (w *window) Add(offset int, val uint64) {
    w.buckets[offset].Add(val)
}

// ResetBucket 重置桶
func (w *window) ResetBucket(offset int) {
    w.buckets[offset%w.size].Reset()
}

// ResetBuckets 批量重置桶
func (w *window) ResetBuckets(offset int, count int) {
    for i := 0; i < count; i++ {
        w.ResetBucket(offset + i)
    }
}

// Reduce 统计有效桶的数量之和
func (w *window) Reduce(start, end int, fn func(*Bucket)) {
    for i := 0; i < end; i++ {
        fn(w.buckets[(start+i)%w.size])
    }
}

func NewWindow(size int) *window {
    buckets := make([]*Bucket, size)
    for index := range buckets {
        buckets[index] = &Bucket{}
    }
    return &window{
        buckets: buckets,
		size:    size,
    }
}
