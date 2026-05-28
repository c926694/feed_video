package app

import "simple_tiktok/internal/modulekit"

func BuildConsumerRunner(modules []modulekit.Module) (*ConsumerRunner, error) {
	runner := NewConsumerRunner()
	for _, module := range modules {
		if err := module.RegisterConsumers(runner); err != nil {
			return nil, err
		}
	}
	return runner, nil
}
