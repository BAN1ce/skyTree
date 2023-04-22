package broker

type Auth = func(username, password string) bool

type UserAuth = func(method string, authData []byte) bool
