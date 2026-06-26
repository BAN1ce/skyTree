//go:build !race
// +build !race

package badger

import (
	"context"
	"errors"
	"testing"
	"time"

	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/google/uuid"
)

func TestLoadMessagePayloadReturnsNotFound(t *testing.T) {
	s, err := NewMessagePayloadStore(t.TempDir(), 1, time.Hour)
	if err != nil {
		t.Fatalf("NewMessagePayloadStore error: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	_, err = s.LoadMessagePayload(context.Background(), uuid.New())
	if !errors.Is(err, brokerstore.ErrMessagePayloadNotFound) {
		t.Fatalf("expected ErrMessagePayloadNotFound, got %v", err)
	}
}

func TestSaveAndLoadMessagePayload(t *testing.T) {
	s, err := NewMessagePayloadStore(t.TempDir(), 1, time.Hour)
	if err != nil {
		t.Fatalf("NewMessagePayloadStore error: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	id := uuid.New()
	want := []byte("hello")
	err = s.SaveMessagePayload(context.Background(), brokerstore.MessagePayloadRecord{
		CreatedAt:         time.Now(),
		MessageID:         id,
		PublishTopic:      "topic/a",
		PublisherClientID: "publisher",
		Payload:           want,
	})
	if err != nil {
		t.Fatalf("SaveMessagePayload error: %v", err)
	}

	got, err := s.LoadMessagePayload(context.Background(), id)
	if err != nil {
		t.Fatalf("LoadMessagePayload error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("payload = %q, want %q", got, want)
	}
}
