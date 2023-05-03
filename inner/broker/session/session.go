package session

import (
	"github.com/BAN1ce/skyTree/pkg"
	"strings"
)

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

func (s *Session) GetWithPrefix(prefix string, keyWithPrefix bool) map[string]string {
	var (
		tmp = make(map[string]string)
	)
	for k, v := range s.m {
		if index := strings.Index(string(k), prefix); index != -1 {
			if keyWithPrefix {
				tmp[string(k[index:])] = v
			} else {
				tmp[string(k)] = v
			}
		}
	}
	return tmp
}
