package providers

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

func TestRetriveInMultitradingEnv_Run_60Times(t *testing.T) {
	t.Parallel()
	for i := 1; i < 60; i++ {
		t.Run("", func(t *testing.T) {
			testRetriveInMultitradingEnv(t, i)
		})
	}
}

func TestRetriveInMultitradingEnv_Single(t *testing.T) {
	testRetriveInMultitradingEnv(t, 7)
}

func TestRetriveInMultitradingEnv_RaceTest_Run_60Times(t *testing.T) {
	for i := 1; i < 60; i++ {
		t.Run("", func(t *testing.T) {
			testRetriveInMultitradingEnvWithoutSleep(t, i)
		})
	}
}

func TestRetriveInMultitradingEnv_Single_RaceTest(t *testing.T) {
	testRetriveInMultitradingEnvWithoutSleep(t, 7)
}

func testRetriveInMultitradingEnv(t *testing.T, quantityJobs int) {
	var provider *vaultConfigProvider
	var countCalls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		time.Sleep(30 * time.Millisecond)

		atomic.AddInt32(&countCalls, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer ts.Close()

	var err error

	provider, err = configureVaultProvider(ts.URL)

	if err != nil {
		t.Fatalf("Erorr configuration %v", err)
	}

	job := func(i int) error {
		provider.Retrieve()
		return nil
	}

	spawn(job, quantityJobs, quantityJobs)

	if countCalls != 1 {
		t.Fatalf("There is %d requests to server, was expected only 1", countCalls)
	}
}

func testRetriveInMultitradingEnvWithoutSleep(t *testing.T, quantityJobs int) {
	var provider *vaultConfigProvider
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
	defer ts.Close()

	var err error

	provider, err = configureVaultProvider(ts.URL)

	if err != nil {
		t.Fatalf("Erorr configuration %v", err)
	}

	job := func(i int) error {
		provider.Retrieve()
		return nil
	}

	spawn(job, quantityJobs, quantityJobs)
}

func configureVaultProvider(url string) (*vaultConfigProvider, error) {
	cfg := DefaultVaultConfig("", "", "", "", url).SetClient(&http.Client{
		Timeout: 20 * time.Second,

		Transport: &http.Transport{

			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	})

	client, err := api.NewClient(&api.Config{Address: cfg.url, HttpClient: cfg.client})
	if err != nil {
		return nil, err
	}

	return &vaultConfigProvider{
		cfg:          &cfg,
		api:          client,
		communicator: make(chan error),
	}, nil
}

type job func(int) error

func spawn(j job, quantityJobs, quantityGoorutines int) {
	jobs := make([]job, quantityJobs)
	for i := 0; i < quantityJobs; i++ {
		jobs[i] = j
	}

	pool(jobs, quantityGoorutines)
}

// pool run all tasks on number goorutines
// and wait for each of them
func pool(tasks []job, number int) (err error) {
	// creating pool of goorutines
	jobs := make(chan job, number)
	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < number; i++ {
		go func(n int) {
			for {
				select {
				case <-ctx.Done():
					return
				case j, ok := <-jobs:
					if !ok {
						return
					}

					j(n)
				}
			}
		}(i)
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(tasks))
	for i := range tasks {
		jobs <- func(i int, wg *sync.WaitGroup) job {
			return func(n int) error {
				defer wg.Done()
				return tasks[i](n)
			}
		}(i, wg)
	}
	wg.Wait()
	cancel()

	return
}
