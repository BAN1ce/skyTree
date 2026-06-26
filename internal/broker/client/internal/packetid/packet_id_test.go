package packetid

import (
	"sync"
	"testing"
)

type packetIDResult struct {
	id  map[uint16]struct{}
	mux sync.RWMutex
}

func (p *packetIDResult) setPacketIDIfNotExists(id uint16) bool {
	p.mux.Lock()
	defer p.mux.Unlock()

	if _, ok := p.id[id]; !ok {
		p.id[id] = struct{}{}
		return true
	}
	return false

}

func TestPacketIDFactory_NextPacketID(t *testing.T) {
	var (
		packetIDResult = &packetIDResult{
			id: map[uint16]struct{}{},
		}
		f = NewPacketIDFactory()
	)

	f.SetID(0)

	for i := 0; i < 50000; i++ {
		go func() {
			id := f.NextPacketID()
			if !packetIDResult.setPacketIDIfNotExists(id) {
				t.Errorf("packetID %d is duplicate", id)
			}

		}()
	}

}

func TestPacketIDToString(t *testing.T) {
	type args struct {
		id uint16
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "toString",
			args: args{
				id: 1,
			},
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PacketIDToString(tt.args.id); got != tt.want {
				t.Errorf("PacketIDToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringToPacketID(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		name string
		args args
		want uint16
	}{
		{
			name: "toPacketID",
			args: args{
				id: "1",
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringToPacketID(tt.args.id); got != tt.want {
				t.Errorf("StringToPacketID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPacketIDTopic_SePacketIDTopic(t *testing.T) {
	var (
		p = NewPacketIDTopic()
	)

	p.SetPacketIDTopic(1, "test")
	if p.GetTopic(1) != "test" {
		t.Error("set packet id topic failed")
	}
}

func TestPacketIDTopic_DeletePacketID(t *testing.T) {
	var (
		p = NewPacketIDTopic()
	)

	p.SetPacketIDTopic(1, "test")
	p.DeletePacketID(1)
	if p.GetTopic(1) != "" {
		t.Error("delete packet id failed")
	}
}
