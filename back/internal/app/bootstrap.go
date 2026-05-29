package app

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/modules"
	"simple_tiktok/internal/svc"

	"github.com/gin-gonic/gin"
)

func BuildModules(ctx *svc.ServiceContext) ([]modulekit.Module, error) {
	return modules.Build(ctx)
}

func BuildHTTPFromContext(ctx *svc.ServiceContext) (*gin.Engine, error) {
	moduleList, err := BuildModules(ctx)
	if err != nil {
		return nil, err
	}
	return BuildHTTPServer(moduleList)
}

func BuildConsumersFromContext(ctx *svc.ServiceContext) (*ConsumerRunner, error) {
	moduleList, err := BuildModules(ctx)
	if err != nil {
		return nil, err
	}
	return BuildConsumerRunner(moduleList)
}
