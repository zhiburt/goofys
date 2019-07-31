package providers

import (
	"net/http"
	"time"
)

// A VaultConfig stores values for vaultConfigProvider
//
// expiredTime is a duration after which time vaultConfigProvider updates secretes
type VaultConfig struct {
	expiredTime    time.Duration
	token          string
	pathToSecrets  string
	accessVaultKey string
	secretVaultKey string
	url            string
	client         *http.Client
}

// DefaultVaultConfig returns config with some default fields
func DefaultVaultConfig(token, pathToSecrets, accessVaultKey, secretVaultKey, address string) VaultConfig {
	cfg := VaultConfig{}.
		SetInformation(token, pathToSecrets, accessVaultKey, secretVaultKey).
		SetEndpoint(address).
		SetTimeExperation(6 * time.Second).
		SetClient(&http.Client{
			Timeout: 5 * time.Second,
		})

	return cfg
}

// SetClient renew new client to cfg
func (cfg VaultConfig) SetClient(client *http.Client) VaultConfig {
	cfg.client = client

	return cfg
}

// SetEndpoint renovate new endpoint to cfg
func (cfg VaultConfig) SetEndpoint(addr string) VaultConfig {
	cfg.url = addr

	return cfg
}

// SetInformation renovate information about vault
func (cfg VaultConfig) SetInformation(token, pathToSecrets, accessVaultKey, secretVaultKey string) VaultConfig {
	cfg.pathToSecrets = pathToSecrets
	cfg.token = token

	cfg.accessVaultKey = accessVaultKey
	cfg.secretVaultKey = secretVaultKey

	return cfg
}

// SetTimeExperation renovate expiredTime to cfg
func (cfg VaultConfig) SetTimeExperation(expiredTime time.Duration) VaultConfig {
	cfg.expiredTime = expiredTime

	return cfg
}
