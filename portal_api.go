package apiplexy

import (
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"
)

func NewPortalAPI() *web.Mux {
	api := web.New()
	api.Use(middleware.SubRouter)
	return api
}
