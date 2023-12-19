package share

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/eclipse/paho.golang/packets"
)

type Client struct {
}

func (c Client) Sub(options packets.SubOptions, client broker.ShareClient) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) UnSub(topic string, clientID string) error {
	//TODO implement me
	panic("implement me")
}

func (c Client) Close() error {
	//TODO implement me
	panic("implement me")
}
