package core

import (
	"net"
	"reflect"
	"testing"

	"github.com/BAN1ce/skyTree/config"
	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay/memory"
)

func TestBrokerClientOptionsPassWillDelayCenter(t *testing.T) {
	center := memory.New()
	b := &Broker{
		config: brokerConfigSet{
			broker: config.Broker{KeepAlive: 30},
		},
		will: brokerWillResources{
			delayCenter: center,
		},
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := brokerclient.NewClient(c1, b.clientOptions()...)
	component := reflect.ValueOf(cl).Elem().FieldByName("component")
	if component.IsNil() {
		t.Fatal("expected client component")
	}
	willDelayCenter := component.Elem().FieldByName("willDelayCenter")
	if willDelayCenter.IsNil() {
		t.Fatal("expected broker client options to pass will delay center")
	}
}
