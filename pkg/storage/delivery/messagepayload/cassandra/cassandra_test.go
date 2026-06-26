package cassandra

import (
	"testing"
	"time"
)

func TestInsertMessagePayloadStatementUsesTTLWhenConfigured(t *testing.T) {
	stmt, ttlSeconds := insertMessagePayloadStatement(2 * time.Hour)

	if ttlSeconds != 7200 {
		t.Fatalf("ttlSeconds = %d, want 7200", ttlSeconds)
	}
	want := `INSERT INTO message_body (message_id, created_ts, publish_topic, publisher_client_id, payload) VALUES (?, ?, ?, ?, ?) USING TTL ?`
	if stmt != want {
		t.Fatalf("statement = %q, want %q", stmt, want)
	}
}

func TestInsertMessagePayloadStatementOmitsTTLWhenDisabled(t *testing.T) {
	stmt, ttlSeconds := insertMessagePayloadStatement(0)

	if ttlSeconds != 0 {
		t.Fatalf("ttlSeconds = %d, want 0", ttlSeconds)
	}
	want := `INSERT INTO message_body (message_id, created_ts, publish_topic, publisher_client_id, payload) VALUES (?, ?, ?, ?, ?)`
	if stmt != want {
		t.Fatalf("statement = %q, want %q", stmt, want)
	}
}
