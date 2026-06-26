package storeruntime

import (
	"strings"
	"testing"

	"github.com/BAN1ce/skyTree/config"
)

// TestCreateSingleNodeKeyStoreRejectsUnsupportedDriver 验证未知 keystore 类型会被拒绝。
func TestCreateSingleNodeKeyStoreRejectsUnsupportedDriver(t *testing.T) {
	_, err := CreateSingleNodeKeyStore(
		config.Store{Default: "unknown_driver"},
		t.TempDir(),
	)
	if err == nil {
		t.Fatal("expected unsupported storage.driver to fail")
	}
	if !strings.Contains(err.Error(), "unsupported storage.driver") {
		t.Fatalf("unexpected error: %v", err)
	}
}
