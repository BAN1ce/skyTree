package metric

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNormalizeSemanticMetricLabels(t *testing.T) {
	tests := []struct {
		name              string
		ownerPath         string
		ownerAction       string
		remoteResult      string
		remoteFailure     string
		rollbackReason    string
		semanticResult    string
		duplicateStage    string
		closeClientStage  string
		closeClientPath   string
		closeClientReason string
		clientStageResult string
		serverStageResult string
		wantOwnerPath     string
		wantOwnerAction   string
		wantRemote        string
		wantFailure       string
		wantRollback      string
		wantResult        string
		wantStage         string
		wantClientStage   string
		wantClientPath    string
		wantClientReason  string
		wantStageResult   string
		wantServerResult  string
	}{
		{
			name:              "known labels pass through",
			ownerPath:         "remote_close",
			ownerAction:       "skip_close",
			remoteResult:      "owner_conflict",
			remoteFailure:     "close_error",
			rollbackReason:    "processing_timeout",
			semanticResult:    "skip",
			duplicateStage:    "ack",
			closeClientStage:  "rpc_invoke",
			closeClientPath:   "dial_new_conn",
			closeClientReason: "deadline_exceeded",
			clientStageResult: "timeout",
			serverStageResult: "owner_conflict",
			wantOwnerPath:     "remote_close",
			wantOwnerAction:   "skip_close",
			wantRemote:        "owner_conflict",
			wantFailure:       "close_error",
			wantRollback:      "processing_timeout",
			wantResult:        "skip",
			wantStage:         "ack",
			wantClientStage:   "rpc_invoke",
			wantClientPath:    "dial_new_conn",
			wantClientReason:  "deadline_exceeded",
			wantStageResult:   "timeout",
			wantServerResult:  "owner_conflict",
		},
		{
			name:              "unknown labels fall back to bounded values",
			ownerPath:         "client-a",
			ownerAction:       "token-a",
			remoteResult:      "node-1",
			remoteFailure:     "client-notify",
			rollbackReason:    "group-a",
			semanticResult:    "client-a",
			duplicateStage:    "message-id",
			closeClientStage:  "socket-a",
			closeClientPath:   "client-a",
			closeClientReason: "dns timeout",
			clientStageResult: "late",
			serverStageResult: "server-a",
			wantOwnerPath:     unknownLabel,
			wantOwnerAction:   "reject",
			wantRemote:        "error",
			wantFailure:       "manager_missing",
			wantRollback:      "client_offline",
			wantResult:        "error",
			wantStage:         "runner",
			wantClientStage:   unknownLabel,
			wantClientPath:    unknownLabel,
			wantClientReason:  "unexpected_response",
			wantStageResult:   "error",
			wantServerResult:  "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOwnerTokenPath(tt.ownerPath); got != tt.wantOwnerPath {
				t.Fatalf("normalizeOwnerTokenPath(%q) = %q, want %q", tt.ownerPath, got, tt.wantOwnerPath)
			}
			if got := normalizeOwnerTokenAction(tt.ownerAction); got != tt.wantOwnerAction {
				t.Fatalf("normalizeOwnerTokenAction(%q) = %q, want %q", tt.ownerAction, got, tt.wantOwnerAction)
			}
			if got := normalizeRemoteCloseResult(tt.remoteResult); got != tt.wantRemote {
				t.Fatalf("normalizeRemoteCloseResult(%q) = %q, want %q", tt.remoteResult, got, tt.wantRemote)
			}
			if got := normalizeRemoteCloseFailure(tt.remoteFailure); got != tt.wantFailure {
				t.Fatalf("normalizeRemoteCloseFailure(%q) = %q, want %q", tt.remoteFailure, got, tt.wantFailure)
			}
			if got := normalizeSharedRollbackReason(tt.rollbackReason); got != tt.wantRollback {
				t.Fatalf("normalizeSharedRollbackReason(%q) = %q, want %q", tt.rollbackReason, got, tt.wantRollback)
			}
			if got := normalizeSemanticResult(tt.semanticResult); got != tt.wantResult {
				t.Fatalf("normalizeSemanticResult(%q) = %q, want %q", tt.semanticResult, got, tt.wantResult)
			}
			if got := normalizeDuplicateDeliveryStage(tt.duplicateStage); got != tt.wantStage {
				t.Fatalf("normalizeDuplicateDeliveryStage(%q) = %q, want %q", tt.duplicateStage, got, tt.wantStage)
			}
			if got := normalizeCloseClientStage(tt.closeClientStage); got != tt.wantClientStage {
				t.Fatalf("normalizeCloseClientStage(%q) = %q, want %q", tt.closeClientStage, got, tt.wantClientStage)
			}
			if got := normalizeCloseClientPath(tt.closeClientPath); got != tt.wantClientPath {
				t.Fatalf("normalizeCloseClientPath(%q) = %q, want %q", tt.closeClientPath, got, tt.wantClientPath)
			}
			if got := normalizeCloseClientFailureReason(tt.closeClientReason); got != tt.wantClientReason {
				t.Fatalf("normalizeCloseClientFailureReason(%q) = %q, want %q", tt.closeClientReason, got, tt.wantClientReason)
			}
			if got := normalizeCloseClientStageResult(tt.clientStageResult); got != tt.wantStageResult {
				t.Fatalf("normalizeCloseClientStageResult(%q) = %q, want %q", tt.clientStageResult, got, tt.wantStageResult)
			}
			if got := normalizeCloseClientServerResult(tt.serverStageResult); got != tt.wantServerResult {
				t.Fatalf("normalizeCloseClientServerResult(%q) = %q, want %q", tt.serverStageResult, got, tt.wantServerResult)
			}
		})
	}
}

func TestRecordSemanticMetricsNormalizeLabels(t *testing.T) {
	ownerBefore := testutil.ToFloat64(OwnerTokenConflictsTotal.WithLabelValues(unknownLabel, "reject"))
	remoteBefore := testutil.ToFloat64(RemoteCloseFailuresTotal.WithLabelValues("manager_missing"))
	rollbackBefore := testutil.ToFloat64(SharedRollbackTotal.WithLabelValues("client_offline", "error"))
	timeoutBefore := testutil.ToFloat64(ProcessingTimeoutTotal.WithLabelValues("normal", "error"))
	duplicateBefore := testutil.ToFloat64(DuplicateDeliveryTotal.WithLabelValues("normal", "runner"))
	closeClientPathBefore := testutil.ToFloat64(CloseClientClientPathTotal.WithLabelValues(unknownLabel, "error"))
	closeClientFailureBefore := testutil.ToFloat64(CloseClientClientFailuresTotal.WithLabelValues(unknownLabel, "unexpected_response"))

	RecordOwnerTokenConflict("client-a", "token-a")
	RecordRemoteCloseFailure("client-a")
	RecordSharedRollback("group-a", "client-a")
	RecordProcessingTimeout("tenant-a", "client-a")
	RecordDuplicateDelivery("tenant-a", "message-id")
	RecordRemoteClose("node-a", time.Millisecond)
	RecordDeliveryCursorLag("tenant-a", time.Millisecond)
	RecordCloseClientClientPath("node-a", "late")
	RecordCloseClientClientFailure("node-a", "dns timeout")
	RecordCloseClientClientStage("socket-a", "late", time.Millisecond)
	RecordCloseClientServerStage("server-a", "slow", time.Millisecond)

	if got := testutil.ToFloat64(OwnerTokenConflictsTotal.WithLabelValues(unknownLabel, "reject")); got != ownerBefore+1 {
		t.Fatalf("owner token conflict counter = %v, want %v", got, ownerBefore+1)
	}
	if got := testutil.ToFloat64(RemoteCloseFailuresTotal.WithLabelValues("manager_missing")); got != remoteBefore+1 {
		t.Fatalf("remote close failure counter = %v, want %v", got, remoteBefore+1)
	}
	if got := testutil.ToFloat64(SharedRollbackTotal.WithLabelValues("client_offline", "error")); got != rollbackBefore+1 {
		t.Fatalf("shared rollback counter = %v, want %v", got, rollbackBefore+1)
	}
	if got := testutil.ToFloat64(ProcessingTimeoutTotal.WithLabelValues("normal", "error")); got != timeoutBefore+1 {
		t.Fatalf("processing timeout counter = %v, want %v", got, timeoutBefore+1)
	}
	if got := testutil.ToFloat64(DuplicateDeliveryTotal.WithLabelValues("normal", "runner")); got != duplicateBefore+1 {
		t.Fatalf("duplicate delivery counter = %v, want %v", got, duplicateBefore+1)
	}
	if got := testutil.ToFloat64(CloseClientClientPathTotal.WithLabelValues(unknownLabel, "error")); got != closeClientPathBefore+1 {
		t.Fatalf("close client path counter = %v, want %v", got, closeClientPathBefore+1)
	}
	if got := testutil.ToFloat64(CloseClientClientFailuresTotal.WithLabelValues(unknownLabel, "unexpected_response")); got != closeClientFailureBefore+1 {
		t.Fatalf("close client failure counter = %v, want %v", got, closeClientFailureBefore+1)
	}
}
