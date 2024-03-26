package topic

type InfoRequest struct {
	Topic string `uri:"topic" binding:"required"`
}
