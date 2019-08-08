package providers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

const itDoesnotMatter = -1

func BenchmarkRetrive(b *testing.B) {
	ts := configureDefaultHTTPServer([]byte(""), http.StatusOK, 0, nil)
	defer ts.Close()

	provider, _ := configureVaultProvider(ts.URL)

	for i := 0; i < b.N; i++ {
		provider.Retrieve()
	}
}

func BenchmarkRetrive_Parallel(b *testing.B) {
	ts := configureDefaultHTTPServer([]byte(""), http.StatusOK, 0, nil)
	defer ts.Close()

	provider, _ := configureVaultProvider(ts.URL)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			provider.Retrieve()
		}
	})
}

func TestRetriveInMultitradingEnv_Run_60Times(t *testing.T) {
	for i := 1; i < 60; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			testRetriveInMultitradingEnv(t, i, 50*time.Millisecond, 1)
		})
	}
}

func TestRetriveInMultitradingEnv_Single(t *testing.T) {
	testRetriveInMultitradingEnv(t, 7, 50*time.Millisecond, 1)
}

func TestRetriveInMultitradingEnv_RaceTest_Run_60Times(t *testing.T) {
	for i := 1; i < 60; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			testRetriveInMultitradingEnv(t, i, 0, itDoesnotMatter)
		})
	}
}

func TestRetriveInMultitradingEnv_Single_RaceTest(t *testing.T) {
	testRetriveInMultitradingEnv(t, 7, 0, itDoesnotMatter)
}

func testRetriveInMultitradingEnv(t *testing.T, quantityJobs int, serversSleepMs time.Duration, expectedConnectionsToServer int32) {
	var countCalls int32

	responce, _ := json.Marshal(struct {
		k string
		v string
	}{"some_key", "some_value"})

	ts := configureDefaultHTTPServer(responce, http.StatusOK, serversSleepMs, func() {
		atomic.AddInt32(&countCalls, 1)
	})
	defer ts.Close()

	provider, err := configureVaultProvider(ts.URL)
	if err != nil {
		t.Fatalf("Erorr configuration %v", err)
	}

	spawn(quantityJobs, quantityJobs, func(i int) error {
		provider.Retrieve()
		return nil
	})

	if expectedConnectionsToServer == itDoesnotMatter {
		return
	}

	if countCalls != expectedConnectionsToServer {
		t.Fatalf("There is %d requests to server, was expected only %v", countCalls, expectedConnectionsToServer)
	}
}

func configureDefaultHTTPServer(body []byte, code int, emulation time.Duration, beforeSending func()) *httptest.Server {
	if beforeSending == nil {
		beforeSending = func() {}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(emulation)

		beforeSending()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}))
}

func configureVaultProvider(url string) (*vaultConfigProvider, error) {
	cfg := NewProviderConfig("", "", "", "", url).SetClient(&http.Client{
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

func spawn(quantityJobs, quantityGoorutines int, j job) {
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
