package scyllastore

import (
	"context"

	"github.com/gocql/gocql"
)

var errNotFound = gocql.ErrNotFound

type cqlSession interface {
	Query(stmt string, values ...any) cqlQuery
	Close()
}

type cqlQuery interface {
	WithContext(ctx context.Context) cqlQuery
	Exec() error
	Scan(dest ...any) error
	Iter() cqlIter
	MapScanCAS(dest map[string]any) (bool, error)
}

type cqlIter interface {
	Scan(dest ...any) bool
	Close() error
}

type gocqlSession struct {
	session *gocql.Session
}

func (s *gocqlSession) Query(stmt string, values ...any) cqlQuery {
	return &gocqlQuery{query: s.session.Query(stmt, values...)}
}

func (s *gocqlSession) Close() {
	if s == nil || s.session == nil {
		return
	}
	s.session.Close()
}

type gocqlQuery struct {
	query *gocql.Query
}

func (q *gocqlQuery) WithContext(ctx context.Context) cqlQuery {
	q.query = q.query.WithContext(ctx)
	return q
}

func (q *gocqlQuery) Exec() error {
	return q.query.Exec()
}

func (q *gocqlQuery) Scan(dest ...any) error {
	return q.query.Scan(dest...)
}

func (q *gocqlQuery) Iter() cqlIter {
	return q.query.Iter()
}

func (q *gocqlQuery) MapScanCAS(dest map[string]any) (bool, error) {
	return q.query.MapScanCAS(dest)
}
