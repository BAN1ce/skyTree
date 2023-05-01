package errs

import "errors"

var (
	ErrListenerIsNil        = errors.New("listener is nil")
	ErrCloseListenerTimeout = errors.New("close listener timeout")
	ErrServerStarted        = errors.New("server has started")
	ErrServerNotStarted     = errors.New("server not started")
)

var (
	ErrorInvalidPacket        = errors.New("invalid packet")
	ErrConnectPacketDuplicate = errors.New("connect packet duplicate")
	ErrProtocolNotSupport     = errors.New("protocol not support")
	ErrPasswordWrong          = errors.New("password wrong")
	ErrAuthHandlerNotSet      = errors.New("auth handler not set")
)
