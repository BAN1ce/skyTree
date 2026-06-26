package acl

import "context"

// Source loads ACL ruleset from a backend.
type Source interface {
	Load(ctx context.Context) (*Ruleset, error)
}
