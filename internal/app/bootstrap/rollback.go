package bootstrap

import (
	"errors"
	"io"
)

// rollbackStack 保存构建阶段已创建资源，用于失败时按反向顺序回滚。
type rollbackStack struct {
	closers []io.Closer
}

// Add 将非空关闭器加入回滚栈。
func (s *rollbackStack) Add(closers ...io.Closer) {
	for _, closer := range closers {
		if closer == nil {
			continue
		}
		s.closers = append(s.closers, closer)
	}
}

// Close 按反向顺序关闭回滚栈中的资源。
func (s *rollbackStack) Close() error {
	return closeAllReverse(s.closers)
}

// closeAllReverse 按切片反向顺序关闭资源并聚合错误。
func closeAllReverse(closers []io.Closer) error {
	var err error
	for i := len(closers) - 1; i >= 0; i-- {
		closer := closers[i]
		if closer == nil {
			continue
		}
		if closeErr := closer.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}
