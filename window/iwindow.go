package window

import "time"

type (
	Window interface {
		Add(offset int, val uint64)
		ResetBuckets(offset int, count int)
		Reduce(start int, end int, fn func(*Bucket))
	}

	Rolling interface {
		Add(val uint64)
		timespan() int
		Reduce(func(*Bucket))
		Summary() (accept uint64, count uint64)
	}

	options struct {
		BucketDuration time.Duration
		Bucket         int
	}

	Option func(*options)
)
