package qos

import (
	"container/list"
	"github.com/eclipse/paho.golang/packets"
)

type PubTask struct {
	messageID string
	packet    *packets.Publish
	retryKey  string
}

func (t *PubTask) SetDuplicate() {
	t.packet.Duplicate = true
}

func (t *PubTask) GetPacket() *packets.Publish {
	return t.packet
}

type PublishQueue struct {
	list *list.List
}

func NewPublishQueue() *PublishQueue {
	return &PublishQueue{
		list: list.New(),
	}
}

func (q *PublishQueue) PushBack(task *PubTask) {
	q.list.PushBack(task)
}

func (q *PublishQueue) Front() *PubTask {
	if tmp := q.list.Front(); tmp != nil {
		return tmp.Value.(*PubTask)
	}
	return nil
}

func (q *PublishQueue) Remove(task *PubTask) {
	for e := q.list.Front(); e != nil; e = e.Next() {
		if e.Value.(*PubTask) == task {
			q.list.Remove(e)
			break
		}
	}
}

func (q *PublishQueue) Range(f func(task *PubTask) bool) {
	for e := q.list.Front(); e != nil; e = e.Next() {
		if !f(e.Value.(*PubTask)) {
			break
		}
	}
}
