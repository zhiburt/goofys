package providers

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/vault/api"
)

func NewVaultConfigProvider(t time.Duration) credentials.Provider {
	return &vaultConfigProvider{
		expairedFiredIn: time.Now().Add(t),
		expairedTime:    t,
	}
}

type vaultConfigProvider struct {
	expairedFiredIn time.Time
	expairedTime    time.Duration
}

// TODO: thinking about multitrading behavior
func (c *vaultConfigProvider) Retrieve() (credentials.Value, error) {
	creds := (*vaultConfigProvider).provider(c)
	c.expairedFiredIn = time.Now().Add(c.expairedTime)

	return creds, nil
}

func (c *vaultConfigProvider) IsExpired() bool {
	if time.Now().After(c.expairedFiredIn) {
		return true
	}

	return false
}

// TODO: should have document comments and return err type
func (c *vaultConfigProvider) provider() credentials.Value {
	// TODO: should be provided from flags
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// TODO: should be provided from flags
	client, err := api.NewClient(&api.Config{Address: "https://vault.awsdev.idt.net:8200", HttpClient: httpClient})
	if err != nil {
		return credentials.Value{}
	}

	// TODO: should be provided from flags
	client.SetToken("")

	data, err := client.Logical().Read("secret/n2p/rtcctl/pars/amazon")
	if err != nil {
		return credentials.Value{}
	}

	// TODO: check data values before using
	// cause they can be equal nil
	return credentials.Value{
		data.Data["aws_access_key_id"].(string),
		data.Data["aws_secret_access_key"].(string),
		"",
		"vaultConfigProvider",
	}
}
