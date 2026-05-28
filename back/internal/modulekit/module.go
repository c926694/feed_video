package modulekit

import "github.com/gin-gonic/gin"

type Module interface {
	RegisterHTTP(r *gin.Engine) error
	RegisterConsumers(registrar ConsumerRegistrar) error
}
