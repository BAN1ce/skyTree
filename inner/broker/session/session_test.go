package session

import (
	"errors"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/mock"
	"github.com/golang/mock/gomock"
	"reflect"
	"testing"
)

func Test_clientKey(t *testing.T) {
	type args struct {
		clientID string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				clientID: "",
			},
			want: KeyClientPrefix,
		},
		{
			name: "client1",
			args: args{
				clientID: "client1",
			},
			want: KeyClientPrefix + "client1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientKey(tt.args.clientID).String(); got != tt.want {
				t.Errorf("clientKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_withClientKey(t *testing.T) {
	type args struct {
		key      string
		clientID string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				key:      "",
				clientID: "",
			},
			want: KeyClientPrefix + "/",
		},
		{
			name: "client1",
			args: args{
				key:      "",
				clientID: "client1",
			},
			want: KeyClientPrefix + "client1/",
		},
		{
			name: "client1&key1",
			args: args{
				key:      "key1",
				clientID: "client1",
			},
			want: KeyClientPrefix + "client1" + "/" + "key1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withClientKey(tt.args.key, tt.args.clientID); got != tt.want {
				t.Errorf("withClientKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_Release(t *testing.T) {
	var (
		ctl      = gomock.NewController(t)
		clientID = "123"
	)

	defer ctl.Finish()
	mockSessionStore := mock.NewMockSessionStore(ctl)
	mockSessionStore.EXPECT().DeleteKey(gomock.Any(), "").Return(errors.New("delete key is empty")).AnyTimes()
	mockSessionStore.EXPECT().DeleteKey(gomock.Any(), gomock.Eq(clientKey(clientID).String())).Return(nil).AnyTimes()

	sessionStore := pkg.NewSessionStoreWithTimout(mockSessionStore, 10)
	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "empty key",
			fields: fields{
				clientID: "123",
				store:    sessionStore,
			},
		},
		{
			name: "with key",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			s.Release()
		})
	}
}

func TestSession_ReadSubTopics(t *testing.T) {
	var (
		ctl       = gomock.NewController(t)
		clientID  = "123"
		subTopics = map[string]string{
			clientSubTopicKey(clientID, "123"):  "1",
			clientSubTopicKey(clientID, "1234"): "2",
		}
	)
	defer ctl.Finish()
	mockSessionStore := mock.NewMockSessionStore(ctl)
	mockSessionStore.EXPECT().ReadPrefixKey(gomock.Any(), gomock.Eq(clientSubTopicKeyPrefix(clientID))).Return(subTopics, nil).AnyTimes()
	sessionStore := pkg.NewSessionStoreWithTimout(mockSessionStore, 10)
	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	tests := []struct {
		name       string
		fields     fields
		wantTopics map[string]int32
	}{
		{
			name: "success",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			wantTopics: map[string]int32{
				"123":  1,
				"1234": 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			if gotTopics := s.ReadSubTopics(); !reflect.DeepEqual(gotTopics, tt.wantTopics) {
				t.Errorf("ReadSubTopics() = %v, want %v", gotTopics, tt.wantTopics)
			}
		})
	}
}

func TestSession_CreateSubTopic(t *testing.T) {
	var (
		ctl      = gomock.NewController(t)
		clientID = "client_123"
		topic    = "topic_1"
	)
	defer ctl.Finish()
	mockSessionStore := mock.NewMockSessionStore(ctl)
	mockSessionStore.EXPECT().PutKey(gomock.Any(), gomock.Eq(clientSubTopicKey(clientID, topic)), gomock.Eq("1")).Return(nil).AnyTimes()
	sessionStore := pkg.NewSessionStoreWithTimout(mockSessionStore, 10)
	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	type args struct {
		topic string
		qos   int32
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "empty",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic: "",
				qos:   0,
			},
		},
		{
			name: "success",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic: topic,
				qos:   1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			s.CreateSubTopic(tt.args.topic, tt.args.qos)
		})
	}
}

func TestSession_DeleteSubTopic(t *testing.T) {
	var (
		ctl      = gomock.NewController(t)
		clientID = "client_123"
		topic    = "topic_1"
	)
	defer ctl.Finish()
	mockSessionStore := mock.NewMockSessionStore(ctl)
	mockSessionStore.EXPECT().DeleteKey(gomock.Any(), gomock.Eq(clientSubTopicKey(clientID, topic))).Return(nil).AnyTimes()
	sessionStore := pkg.NewSessionStoreWithTimout(mockSessionStore, 10)
	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	type args struct {
		topic string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "success",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic: topic,
			},
		},
		{
			name: "empty",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			s.DeleteSubTopic(tt.args.topic)
		})
	}
}

func TestSession_ReadTopicUnAckMessageID(t *testing.T) {
	var (
		ctl       = gomock.NewController(t)
		clientID  = "client_123"
		topic     = "topic_1"
		messageID = []string{
			"1",
			"2",
		}
		keyValue = map[string]string{}
	)
	for _, id := range messageID {
		keyValue[clientTopicUnAckKey(clientID, topic, id)] = id
	}
	defer ctl.Finish()
	mockSessionStore := mock.NewMockSessionStore(ctl)
	mockSessionStore.EXPECT().ReadPrefixKey(gomock.Any(), gomock.Eq(clientTopicUnAckKeyPrefix(clientID, topic))).Return(keyValue, nil).AnyTimes()
	sessionStore := pkg.NewSessionStoreWithTimout(mockSessionStore, 10)

	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	type args struct {
		topic string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		wantId []string
	}{
		{
			name: "success",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic: topic,
			},
			wantId: messageID,
		},
		{
			name: "empty",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic: "",
			},
			wantId: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			if gotId := s.ReadTopicUnAckMessageID(tt.args.topic); !reflect.DeepEqual(gotId, tt.wantId) {
				t.Errorf("ReadTopicUnAckMessageID() = %v, want %v", gotId, tt.wantId)
			}
		})
	}
}

func TestSession_CreateTopicUnAckMessageID(t *testing.T) {
	var (
		ctl       = gomock.NewController(t)
		clientID  = "client_123"
		topic     = "topic_1"
		messageID = []string{
			"1",
			"2",
		}
		keyValue = map[string]string{}
	)
	for _, id := range messageID {
		keyValue[clientTopicUnAckKey(clientID, topic, id)] = id
	}
	defer ctl.Finish()
	mockSessionStore := mock.NewMockSessionStore(ctl)
	mockSessionStore.EXPECT().PutKey(gomock.Any(), gomock.Eq(clientTopicUnAckKey(clientID, topic, messageID[0])), gomock.Eq(messageID[0])).Return(nil)
	mockSessionStore.EXPECT().PutKey(gomock.Any(), gomock.Eq(clientTopicUnAckKey(clientID, topic, messageID[1])), gomock.Eq(messageID[1])).Return(nil)
	sessionStore := pkg.NewSessionStoreWithTimout(mockSessionStore, 10)
	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	type args struct {
		topic     string
		messageID []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "message_1",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic:     topic,
				messageID: messageID,
			},
		},
		{
			name: "empty",
			fields: fields{
				clientID: clientID,
				store:    sessionStore,
			},
			args: args{
				topic:     "",
				messageID: messageID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			s.CreateTopicUnAckMessageID(tt.args.topic, tt.args.messageID)
		})
	}
}

func TestSession_DeleteTopicUnAckMessageID(t *testing.T) {
	type fields struct {
		clientID string
		store    *pkg.SessionStoreWithTimeout
	}
	type args struct {
		topic     string
		messageID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "",
			fields: fields{},
			args:   args{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				clientID: tt.fields.clientID,
				store:    tt.fields.store,
			}
			s.DeleteTopicUnAckMessageID(tt.args.topic, tt.args.messageID)
		})
	}
}
