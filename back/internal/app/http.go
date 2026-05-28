package app

import (
	"simple_tiktok/internal/modulekit"

	"github.com/gin-gonic/gin"
)

func BuildHTTPServer(modules []modulekit.Module) (*gin.Engine, error) {
	r := gin.Default()
	for _, module := range modules {
		if err := module.RegisterHTTP(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}
