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
	// ErrInformationFromVault is the error code for errors when vault not give any information
	ErrInformationFromVault = errors.New("Information from vault is nil")
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
	communicator    chan error
	needNotify      int32
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
		communicator:    make(chan error),
	}

	provider.initSecrets()

	return provider, nil
}

// Retrieve goes to vault for new secrets
// if something wrong happend it guarantees provider has the same state as before
//
// TODO: make up the working way of this function
func (c *vaultConfigProvider) Retrieve() (creds credentials.Value, err error) {
	// This lock must to be here, since
	// some of goorutines can fall into first branch and that can couse undefiend behaiviour
	// we must ensure that if statement is threadesafe.
	//
	// We could use atomic here
	// but initial mutex should be also remaine
	// because we need guaranteing that when first goorutine in defer from second branch
	// no any others will trying to increment needNotify.
	//
	// More information:
	// One situation when can intiate data race when we do atomic.StoreInt32(&c.needNotify, 0)
	// it's gonna be problem if someone update c.needNotify even it would be atomic operation
	// this goorution can dangle for endless time
	//
	// it's becouse we refuse atomic this code approach
	c.Lock()
	c.needNotify++

	if c.needNotify != 1 {
		c.Unlock() // Unlock first lock

		err = <-c.communicator
		if err != nil {
			return creds, err
		}
	} else {
		defer func() {
			c.Lock()
			c.expairedFiredIn = time.Now().Add(c.cfg.expiredTime)

			for i := int32(0); i < c.needNotify-1; i++ {
				c.communicator <- err
			}

			c.needNotify = 0
			c.Unlock()
		}()

		c.Unlock() // Unlock first lock

		var secretes map[string]interface{}
		secretes, err = c.provide()
		if err != nil {
			return creds, ErrConnectionToVault
		}

		err = c.fillUpSecrets(secretes)
		if err != nil {

			return creds, ErrConnectionToVault
		}
	}

	return c.getCreds(), nil
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

	if data == nil {
		return nil, ErrInformationFromVault
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
	c.RLock()
	for key := range c.secrets {
		if secret, ok := data[key]; !ok {
			return fmt.Errorf("There is no secret with key: %v", key)
		} else {
			if _, isString := secret.(string); !isString {
				return fmt.Errorf("Secret has wrong type key: %v", key)
			}
		}
	}
	c.RUnlock()

	c.Lock()
	for key := range c.secrets {
		c.secrets[key] = data[key].(string)
	}
	c.Unlock()

	return nil
}

// getCreds gets credentials from locale map
func (c *vaultConfigProvider) getCreds() credentials.Value {
	c.RLock()
	defer c.RUnlock()

	return credentials.Value{
		AccessKeyID:     c.secrets[c.cfg.accessVaultKey],
		SecretAccessKey: c.secrets[c.cfg.secretVaultKey],
		ProviderName:    VaultProviderName,
	}
}

// initSecrets creates main secrets
// the have to be created in any way
func (c *vaultConfigProvider) initSecrets() {
	c.secrets[c.cfg.accessVaultKey] = ""
	c.secrets[c.cfg.secretVaultKey] = ""
}
