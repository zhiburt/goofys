package providers

import (
	"net/http"
	"time"
)

// A ProviderConfig stores values for vaultConfigProvider
//
// expiredTime is a duration after which time vaultConfigProvider updates secretes
type ProviderConfig struct {
	expiredTime    time.Duration
	token          string
	pathToSecrets  string
	accessVaultKey string
	secretVaultKey string
	url            string
	client         *http.Client
}

// NewProviderConfig returns config with some default fields
func NewProviderConfig(token, pathToSecrets, accessVaultKey, secretVaultKey, address string) ProviderConfig {
	cfg := ProviderConfig{}.
		SetInformation(token, pathToSecrets, accessVaultKey, secretVaultKey).
		SetEndpoint(address).
		SetTimeExperation(6 * time.Second).
		SetClient(&http.Client{
			Timeout: 5 * time.Second,
		})

	return cfg
}

// SetClient renew new client to cfg
func (cfg ProviderConfig) SetClient(client *http.Client) ProviderConfig {
	cfg.client = client

	return cfg
}

// SetEndpoint renovate new endpoint to cfg
func (cfg ProviderConfig) SetEndpoint(addr string) ProviderConfig {
	cfg.url = addr

	return cfg
}

// SetInformation renovate information about vault
func (cfg ProviderConfig) SetInformation(token, pathToSecrets, accessVaultKey, secretVaultKey string) ProviderConfig {
	cfg.pathToSecrets = pathToSecrets
	cfg.token = token

	cfg.accessVaultKey = accessVaultKey
	cfg.secretVaultKey = secretVaultKey

	return cfg
}

// SetTimeExperation renovate expiredTime to cfg
func (cfg ProviderConfig) SetTimeExperation(expiredTime time.Duration) ProviderConfig {
	cfg.expiredTime = expiredTime

	return cfg
}
