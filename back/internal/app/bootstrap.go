package app

import (
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/modules"

	"github.com/gin-gonic/gin"
)

func BuildModules(ctx modulekit.Context) ([]modulekit.Module, error) {
	return modules.Build(ctx)
}

func BuildHTTPFromContext(ctx modulekit.Context) (*gin.Engine, error) {
	moduleList, err := BuildModules(ctx)
	if err != nil {
		return nil, err
	}
	return BuildHTTPServer(moduleList)
}

func BuildConsumersFromContext(ctx modulekit.Context) (*ConsumerRunner, error) {
	moduleList, err := BuildModules(ctx)
	if err != nil {
		return nil, err
	}
	return BuildConsumerRunner(moduleList)
}
