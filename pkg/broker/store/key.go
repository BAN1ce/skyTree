package store

const (
	GlobalPrefix = "skyTree"
	RetainPrefix = "retain"
)

func GetTopicRetainKey(topic string) string {
	return GlobalPrefix + "/" + RetainPrefix + "/" + topic
}
