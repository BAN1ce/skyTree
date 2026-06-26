package acl

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type FileSource struct {
	Path string
}

func (s *FileSource) Load(_ context.Context) (*Ruleset, error) {
	if s == nil || s.Path == "" {
		return nil, fmt.Errorf("acl file path is empty")
	}
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	var rs Ruleset
	if err := yaml.Unmarshal(b, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

func FileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
