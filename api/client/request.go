package client

type InfoReq struct {
	ClientID string `uri:"client_id" binding:"required"`
}
