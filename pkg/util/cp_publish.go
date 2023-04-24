package util

import "github.com/eclipse/paho.golang/packets"

func CpPublish(src, des *packets.Publish) {
	des.Topic = src.Topic
	des.Payload = src.Payload
}
