package errs

import "errors"

var (
	ErrListenerIsNil        = errors.New("listener is nil")
	ErrCloseListenerTimeout = errors.New("close listener timeout")
	ErrServerStarted        = errors.New("server has started")
	ErrServerNotStarted     = errors.New("server not started")
)

var (
	ErrInvalidPacket              = errors.New("invalid packet")
	ErrInvalidRequestResponseInfo = errors.New("invalid request response info")
	ErrInvalidRequestProblemInfo  = errors.New("invalid request problem info")
	ErrConnackInvalidClientID     = errors.New("connack invalid client id")
	ErrSetClientSession           = errors.New("set client session error")
	ErrClientIDEmpty              = errors.New("client id empty")
	ErrConnectPacketDuplicate     = errors.New("connect packet duplicate")
	ErrProtocolNotSupport         = errors.New("protocol not support")
	ErrPasswordWrong              = errors.New("password wrong")
	ErrAuthHandlerNotSet          = errors.New("auth handler not set")
)

var (
	ErrInvalidQoS = errors.New("invalid qos")
)
