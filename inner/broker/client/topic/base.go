package topic

import (
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
)

func copyPublish(publish *packets.Publish) *packets.Publish {
	var publishPacket = pool.PublishPool.Get()
	pool.CopyPublish(publishPacket, publish)
	publishPacket.QoS = pkg.QoS0
	return publishPacket
}
