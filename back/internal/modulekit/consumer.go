package modulekit

type ConsumerRegistrar interface {
	Add(name string, start func() error)
}
