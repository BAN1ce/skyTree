package utils

type Bucket struct {
	c   chan struct{}
	num int
}

// NewBucket create a bucket with num tokens
// if num <= 0, bucket is unlimited
func NewBucket(num int) *Bucket {
	var (
		c chan struct{}
	)
	if num <= 0 {
		c = make(chan struct{})
		close(c)
	} else {
		c = make(chan struct{}, num)
		for i := 0; i < num; i++ {
			c <- struct{}{}
		}
	}
	return &Bucket{
		c: c,
	}
}

func (b *Bucket) GetToken() <-chan struct{} {
	return b.c
}

// PutToken put a token into bucket, this operation is not concurrent safe
func (b *Bucket) PutToken() {
	if b.num <= 0 {
		return
	}
	select {
	case b.c <- struct{}{}:
	default:
	}
}
