package rate

import (
	"context"

	"github.com/BAN1ce/skyTree/logger"
)

var (
	// noLimitBucket is a bucket with no limit
	// it's used when rate limit is not needed
	noLimitBucket chan struct{}
)

func init() {
	noLimitBucket = make(chan struct{})
	close(noLimitBucket)
}

type Bucket struct {
	ch  chan struct{}
	num int
}

// NewBucket create a bucket with num tokens
// if num <= 0, bucket is unlimited
func NewBucket(num int) *Bucket {
	var (
		b = &Bucket{
			num: num,
		}
	)
	if num <= 0 {
		b.ch = noLimitBucket
		return b
	}
	b.ch = make(chan struct{}, num)
	for i := 0; i < num; i++ {
		b.ch <- struct{}{}
	}
	return b
}

func (b *Bucket) GetToken(ctx context.Context) {
	select {
	case <-ctx.Done():
		logger.Logger.Debug().Msg("context done")
	case <-b.ch:

	}

}

func (b *Bucket) TryGetToken() bool {
	if b == nil {
		return false
	}
	if b.num <= 0 {
		return true
	}
	select {
	case <-b.ch:
		return true
	default:
		return false
	}
}

// PutToken put a token into bucket, this operation is not concurrent safe
func (b *Bucket) PutToken() {
	if b.num <= 0 {
		return
	}
	select {
	case b.ch <- struct{}{}:
	default:
		logger.Logger.Error().Int("bucket count", len(b.ch)).Msg("put token into bucket failed, bucket is full, token will be dropped. It's abnormal, please check the code.")
	}
}

func (b *Bucket) RemainingToken() int {
	return len(b.ch)
}
