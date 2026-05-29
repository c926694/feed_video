package app

import (
	"simple_tiktok/internal/modulekit"

	"github.com/gin-gonic/gin"
)

func BuildHTTPServer(modules []modulekit.Module) (*gin.Engine, error) {
	r := gin.Default()
	for _, module := range modules {
		next, err := module.RegisterHTTP(r)
		if err != nil {
			return nil, err
		}
		r = next
	}
	return r, nil
}
