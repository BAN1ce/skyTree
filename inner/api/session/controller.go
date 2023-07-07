package session

import (
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/labstack/echo/v4"
)

type Controller struct {
	pkg.Session
}

func (o *Controller) Info(ctx echo.Context) error {
	var (
		req = InfoRequest{}
		err error
	)
	if err = ctx.Bind(&req); err != nil {
		return err
	}
	return err
}
