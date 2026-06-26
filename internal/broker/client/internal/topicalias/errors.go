package topicalias

import "errors"

var (
	ErrTopicAliasNotFound = errors.New("topic alias not found")
	ErrTopicAliasInvalid  = errors.New("topic alias invalid")
)
