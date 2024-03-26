package base

const (
	CodeClientNotExists = 1000 + iota
)

type Response struct {
	Code    int         `json:"code,omitempty"`
	Msg     string      `json:"msg,omitempty"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type Page struct {
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func WithData(data interface{}) *Response {
	return &Response{
		Code:    0,
		Msg:     "",
		Success: true,
		Data:    data,
	}
}

func WithSuccess() *Response {
	return &Response{
		Code:    0,
		Msg:     "",
		Success: true,
	}
}

func WithCode(code int) *Response {
	return &Response{
		Code:    code,
		Msg:     "",
		Success: false,
	}
}

func WithError(err error) *Response {
	// Fixme: use specific error code
	return &Response{
		Code:    500,
		Msg:     err.Error(),
		Success: false,
	}
}
