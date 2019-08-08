package providers

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

var ErrCannotFindProvider = errors.New("Cannot create provider")

type (
	Providers map[string]Configurator

	Configurator func(*ProviderConfig) (credentials.Provider, error)
)

var providers = Providers{
	"vault": NewVaultConfigProvider,
}

func Use(name string) (Configurator, error) {
	if provider, ok := providers[name]; ok {
		return provider, nil
	}

	return nil, ErrCannotFindProvider
}

func NewCredentials(p credentials.Provider) *credentials.Credentials {
	return credentials.NewCredentials(p)
}
