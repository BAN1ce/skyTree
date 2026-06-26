package cmdruntime

import (
	"bytes"
	"testing"
)

func TestStartupReporterPrintVersionInfoUsesReporterWriter(t *testing.T) {
	var out bytes.Buffer
	reporter := NewStartupReporter(&out)

	reporter.PrintVersionInfo("v0.1.0")

	if !bytes.Contains(out.Bytes(), []byte("Version Information")) {
		t.Fatalf("expected version info to be written through reporter, got %q", out.String())
	}
}

func TestBufferedStartupReporterFlushesOnlyWhenRequested(t *testing.T) {
	var out bytes.Buffer
	reporter := NewBufferedStartupReporter(&out)

	reporter.line("hello %s", "skytree")
	if out.Len() != 0 {
		t.Fatalf("expected buffered reporter to hold output before flush, got %q", out.String())
	}

	if err := reporter.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if got := out.String(); got != "hello skytree" {
		t.Fatalf("unexpected flushed output: %q", got)
	}
}
