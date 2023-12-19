package session

type InfoRequest struct {
	ClientID string `param:"client_id" validate:"required"`
}
