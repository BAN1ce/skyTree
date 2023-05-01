package session

import "github.com/BAN1ce/skyTree/pkg"

type Session struct {
	m map[pkg.SessionKey]string
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Set(key pkg.SessionKey, value string) {
	if s.m == nil {
		return
	}
	s.m[key] = value

}
func (s *Session) Get(key pkg.SessionKey) string {
	if s.m == nil {
		return ""
	}
	return s.m[key]
}

func (s *Session) Destroy() {
	s.m = nil
	return
}
