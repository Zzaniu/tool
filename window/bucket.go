package window

type Bucket struct {
	Sum   uint64 // 这里可以是请求耗时, 也可以是请求成功次数
	Count uint64 // 请求总数
}

// Add Count是总数，这个是必须加1的
func (b *Bucket) Add(val uint64) {
	b.Sum += val
	b.Count++
}

// Reset 重置桶
func (b *Bucket) Reset() {
	b.Sum = 0.0
	b.Count = 0
}
