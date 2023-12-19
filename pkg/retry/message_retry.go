package retry

type MessageRetry interface {
	DeleteRetryKey(key string)
	CreatPublishTask(task *Task) error
}
