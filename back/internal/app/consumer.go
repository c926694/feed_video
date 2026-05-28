package app

import "simple_tiktok/internal/modulekit"

type ConsumerRunner struct {
	consumers []consumerEntry
}

type consumerEntry struct {
	name  string
	start func() error
}

func NewConsumerRunner() *ConsumerRunner {
	return &ConsumerRunner{
		consumers: make([]consumerEntry, 0),
	}
}

func (r *ConsumerRunner) Add(name string, start func() error) {
	r.consumers = append(r.consumers, consumerEntry{
		name:  name,
		start: start,
	})
}

var _ modulekit.ConsumerRegistrar = (*ConsumerRunner)(nil)

func (r *ConsumerRunner) StartAll() <-chan error {
	errCh := make(chan error, len(r.consumers))
	for _, consumer := range r.consumers {
		start := consumer.start
		go func() {
			errCh <- start()
		}()
	}
	return errCh
}
