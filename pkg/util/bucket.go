package util

type Bucket struct {
	c chan struct{}
}

func NewBucket(num int) *Bucket {
	var (
		c = make(chan struct{}, num)
	)
	for i := 0; i < num; i++ {
		c <- struct{}{}
	}
	return &Bucket{
		c: c,
	}
}

func (b *Bucket) GetToken() <-chan struct{} {
	return b.c
}

func (b *Bucket) PutToken() {
	select {
	case b.c <- struct{}{}:
	default:

	}
}
