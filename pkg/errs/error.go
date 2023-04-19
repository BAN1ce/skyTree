package errs

import "errors"

var (
	ErrListenerIsNil        = errors.New("listener is nil")
	ErrCloseListenerTimeout = errors.New("close listener timeout")
	ErrServerStarted        = errors.New("server has started")
	ErrServerNotStarted     = errors.New("server not started")
)
