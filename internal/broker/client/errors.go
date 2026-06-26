package client

import "errors"

var (
	ErrClientIDEmpty          = errors.New("client id empty")
	ErrClientUsernameNotEmpty = errors.New("client username not empty")
	ErrClientPasswordNotEmpty = errors.New("client password not empty")
	ErrProtocolError          = errors.New("protocol error")
	ErrAuthHandlerNotSet      = errors.New("auth handler not set")
	ErrStore                  = errors.New("store error")
)
