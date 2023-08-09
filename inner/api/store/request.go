package store

type getRequest struct {
	Page     int    `query:"page"`
	PageSize int    `query:"page_size"`
	Topic    string `query:"store" validate:"required"`
}
