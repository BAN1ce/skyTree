package window

import "context"

type Windows struct {
	ctx    context.Context
	window chan struct{}
}

func NewWindows(ctx context.Context, size int) *Windows {
	w := &Windows{
		ctx:    ctx,
		window: make(chan struct{}, size),
	}
	for i := 0; i < size; i++ {
		w.Put()
	}
	return w
}

func (w *Windows) Get() {
	select {
	case <-w.ctx.Done():
		return
	case <-w.window:
		return
	}
}

func (w *Windows) Put() {
	select {
	case <-w.ctx.Done():
		return
	case w.window <- struct{}{}:
		return
	}
}
