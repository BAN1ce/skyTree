package main

import (
	"os/exec"
	"testing"
)

func TestMainFileCompilesAsCommandEntry(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go", "-h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run main.go -h failed: %v\n%s", err, out)
	}
}
