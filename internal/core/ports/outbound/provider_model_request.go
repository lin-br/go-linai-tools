package outbound

type ProviderModelHandler interface {
	DoMessagesRequest(params string) (string, error)
}
