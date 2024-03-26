package errs

import "errors"

var (
	ErrListenerIsNil        = errors.New("listener is nil")
	ErrCloseListenerTimeout = errors.New("close listener timeout")
	ErrServerStarted        = errors.New("server has started")
	ErrServerNotStarted     = errors.New("server not started")
)

// Client
var (
	ErrInvalidPacket              = errors.New("invalid packet")
	ErrInvalidRequestResponseInfo = errors.New("invalid api response info")
	ErrInvalidRequestProblemInfo  = errors.New("invalid api problem info")
	ErrConnackInvalidClientID     = errors.New("connack invalid client id")
	ErrSetClientSession           = errors.New("set client client.proto error")
	ErrClientIDEmpty              = errors.New("client id empty")
	ErrConnectPacketDuplicate     = errors.New("connect packet duplicate")
	ErrProtocolNotSupport         = errors.New("protocol not support")
	ErrPasswordWrong              = errors.New("password wrong")
	ErrAuthHandlerNotSet          = errors.New("auth handler not set")
	ErrClientClosed               = errors.New("client closed")
	ErrTopicAliasNotFound         = errors.New("topic alias not found")
	ErrTopicAliasInvalid          = errors.New("topic alias invalid")
	ErrPacketOversize             = errors.New("packet oversize")
	ErrSubTopicEmpty              = errors.New("sub topic empty")
)

var (
	ErrInvalidQoS                = errors.New("invalid qos")
	ErrTopicNotExistsInSubTopics = errors.New("topic not exists in sub topics")
	ErrTopicQoSNotSupport        = errors.New("topic qos not support")
)

var (
	ErrStoreMessageLength  = errors.New("store message length error")
	ErrStoreMessageExpired = errors.New("store message expired")
	ErrStoreVersionInvalid = errors.New("store version invalid")
	ErrStoreReadCreateTime = errors.New("read create time failed")
	ErrStoreTopicsEmpty    = errors.New("store topics empty")
	ErrStoreKeyNotFound    = errors.New("store key not found")
)

var (
	ErrSessionConnectPropertiesNotFound = errors.New("session connect properties not found")
	ErrSessionWillMessageNotFound       = errors.New("session will message not found")
)

var (
	ErrClientRemoved = errors.New("client removed")
)
