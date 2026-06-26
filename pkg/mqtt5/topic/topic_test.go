package topic

import (
	"reflect"
	"testing"
)

func TestParseShareTopic(t *testing.T) {
	type args struct {
		shareTopic string
	}
	tests := []struct {
		name           string
		args           args
		wantShareGroup string
		wantSubTopic   string
	}{
		{
			name: "parse",
			args: args{
				shareTopic: "$share/group/topic",
			},
			wantShareGroup: "group",
			wantSubTopic:   "topic",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShareGroup, gotSubTopic := ParseShareTopic(tt.args.shareTopic)
			if gotShareGroup != tt.wantShareGroup {
				t.Errorf("ParseShareTopic() gotShareGroup = %v, want %v", gotShareGroup, tt.wantShareGroup)
			}
			if gotSubTopic != tt.wantSubTopic {
				t.Errorf("ParseShareTopic() gotSubTopic = %v, want %v", gotSubTopic, tt.wantSubTopic)
			}
		})
	}
}

func TestSplitTopic(t *testing.T) {
	type args struct {
		topic string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		//{
		//	name: " /a/b",
		//	args: args{
		//		topic: "/a/b",
		//	},
		//	want: []string{
		//		"/",
		//		"a",
		//		"b",
		//	},
		//},
		//{
		//	name: " a/b",
		//	args: args{
		//		topic: "a/b",
		//	},
		//	want: []string{
		//		"a",
		//		"b",
		//	},
		//},
		//{
		//	name: " a",
		//	args: args{
		//		topic: "a",
		//	},
		//	want: []string{
		//		"a",
		//	},
		//},
		//{
		//	name: "/a",
		//	args: args{
		//		topic: "/a",
		//	},
		//	want: []string{
		//		"/",
		//		"a",
		//	},
		//},
		{
			name: "/",
			args: args{
				topic: "/",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitTopic(tt.args.topic); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitTopic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitTopicLevels_StrictMQTTSemantics(t *testing.T) {
	type tc struct {
		name  string
		topic string
		want  []string
	}
	tests := []tc{
		{
			name:  "no_slash",
			topic: "a",
			want:  []string{"a"},
		},
		{
			name:  "leading_slash",
			topic: "/a/b",
			want:  []string{"", "a", "b"},
		},
		{
			name:  "trailing_slash",
			topic: "a/",
			want:  []string{"a", ""},
		},
		{
			name:  "double_slash",
			topic: "a//b",
			want:  []string{"a", "", "b"},
		},
		{
			name:  "root_topic",
			topic: "/",
			want:  []string{"", ""},
		},
		{
			name:  "empty_topic",
			topic: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitTopicLevels(tt.topic); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitTopicLevels(%q) = %v, want %v", tt.topic, got, tt.want)
			}
		})
	}
}
