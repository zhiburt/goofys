package providers

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/vault/api"
)

// VaultProviderName the name of provider in credentials.Value
const VaultProviderName = "VaultConfigProvider"

var (
	// ErrSecrets is the error code which indicates troubles with secrets
	ErrSecrets = errors.New("There are no secrets we need for or they have wrong type")
	// ErrInitVault is the error code for invalid parameters errors
	// and client of vault creation error
	ErrInitVault = errors.New("Cannot construct vault")
	// ErrConnectionToVault is the error code for errors with vault connection
	ErrConnectionToVault = errors.New("Cannot get into vault")
)

// A vaultConfigProvider ensure a pile of values for vault
// implements credentials.Provider interface to be a provider of credentials
//
// It gets expiration duration it is stored in config's expiredTime field. At the time when it is expired it updates secrets(credentials) from vault
// and update expiration time, this process is repeatable.
//
// expairedFiredIn contains the time after the secrets should be updated
//
// api field is a client to vault
//
// cfg contains information about connection and an expired value
//
// secrets store all secretes which we want to have. The chief ones are AccessKeyID and SecretAccessKey
// gets from config cfg.accessVaultKey and cfg.secretVaultKey accordingly
type vaultConfigProvider struct {
	expairedFiredIn time.Time
	api             *api.Client
	cfg             *VaultConfig
	secrets         map[string]string
	sync.RWMutex
}

// NewVaultConfigProvider return new vault provider using vaultConfig
// where all necessary fields already put
//
// if a connection to vault isn't available it returns InitVaultErr
func NewVaultConfigProvider(cfg *VaultConfig) (credentials.Provider, error) {
	client, err := api.NewClient(&api.Config{Address: cfg.url, HttpClient: cfg.client})
	if err != nil {
		return nil, ErrInitVault
	}

	client.SetToken(cfg.token)

	provider := &vaultConfigProvider{
		expairedFiredIn: time.Now().Add(cfg.expiredTime),
		api:             client,
		cfg:             cfg,
		secrets:         make(map[string]string),
	}

	provider.initSecrets()

	return provider, nil
}

// Retrieve goes to vault for new secrets
// if something wrong happend it guarantees provider has the same state as before
//
// TODO: It would be better if we use chanles here instead of mutex?
// Although we lock all goorutines and one of them goes to vault,
// each of them also requests new information from vault
func (c *vaultConfigProvider) Retrieve() (credentials.Value, error) {
	c.Lock()
	defer c.Unlock()

	secretes, err := c.provide()
	if err != nil {
		return credentials.Value{}, ErrConnectionToVault
	}

	err = c.fillUpSecrets(secretes)
	if err != nil {
		return credentials.Value{}, ErrSecrets
	}

	c.expairedFiredIn = time.Now().Add(c.cfg.expiredTime)

	return credentials.Value{
		AccessKeyID:     c.secrets[c.cfg.accessVaultKey],
		SecretAccessKey: c.secrets[c.cfg.secretVaultKey],
		ProviderName:    VaultProviderName,
	}, nil
}

// IsExpired check the time has come or no
//
// it uses read lock since some of goorutines can change expairedFiredIn at the same time
func (c *vaultConfigProvider) IsExpired() bool {
	c.RLock()
	defer c.RUnlock()
	if time.Now().After(c.expairedFiredIn) {
		return true
	}

	return false
}

// provide create a new request to vault by vault client
func (c *vaultConfigProvider) provide() (map[string]interface{}, error) {
	data, err := c.api.Logical().Read(c.cfg.pathToSecrets)
	if err != nil {
		return nil, err
	}

	return data.Data, nil
}

// fillUpSecrets check fields we involve in and update them in secrets
//
// if there is some problem(invalid type of data or no such value by key),
// the state of provider will the same as before call of this function
// and return error
//
// using 2 cycles to have less memory allocation
// there's a hypothesis that the secretes it is conserned with quit small map.
func (c *vaultConfigProvider) fillUpSecrets(data map[string]interface{}) error {
	for key := range c.secrets {
		if secret, ok := data[key]; !ok {
			return fmt.Errorf("There is no secret with key: %v", key)
		} else {
			if _, isString := secret.(string); !isString {
				return fmt.Errorf("Secret has wrong type key: %v", key)
			}
		}
	}

	for key := range c.secrets {
		c.secrets[key] = data[key].(string)
	}

	return nil
}

// initSecrets creates main secrets
// the have to be created in any way
func (c *vaultConfigProvider) initSecrets() {
	c.secrets[c.cfg.accessVaultKey] = ""
	c.secrets[c.cfg.secretVaultKey] = ""
}
