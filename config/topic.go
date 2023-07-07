package config

type Topic struct {
	WindowSize int
}

func GetTopic() Topic {
	return Topic{
		WindowSize: 10,
	}
}
