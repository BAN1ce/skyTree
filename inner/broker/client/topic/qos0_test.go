package topic

import (
	"github.com/BAN1ce/skyTree/inner/broker/client/topic/mock"
	"github.com/golang/mock/gomock"
	"testing"
)

func TestQoS0_afterClose(t1 *testing.T) {
	var (
		ctrl      = gomock.NewController(t1)
		mockEvent = mock.NewMockPublishListener(ctrl)
		topic     = "topic"
		t         = &QoS0{
			topic:           topic,
			publishListener: mockEvent,
		}
		success bool
	)
	mockEvent.EXPECT().DeletePublishEvent(gomock.Eq(topic), gomock.InAnyOrder(t.handler)).Do(func() {
		success = true
	})
	t.afterClose()
	if !success {
		t1.Error("QoS0.afterClose() failed")
	}
}
