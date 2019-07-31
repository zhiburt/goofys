package providers

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

// we should provide apportunity to provide some URL to as well
// so better configure should be done
// emulate some huge enough delay to all gorutines be asleep
// and check if expairedFiredIn has been changed only one time
// typically it does checking the difference bettween
// it it is too much it made several requests
func TestRetriveInMultitradingEnv(t *testing.T) {
	durationOfUpdates := time.Millisecond * 500
	expectedTime := time.Now().Add(durationOfUpdates)
	provider := &vaultConfigProvider{
		expairedFiredIn: expectedTime,
		expairedTime:    durationOfUpdates,
	}
	f := func(_ int) (err error) {
		provider.Retrieve()
		return
	}

	pool([]job{f, f, f, f, f, f, f}, 3)

	diffBetween := math.Abs(float64(expectedTime.Sub(provider.expairedFiredIn)))
	if diffBetween > 1000.0 {
		t.Fatalf("the difference is too much expected time:\n%s\nwas:\n%s\ndifference:%f\n",
			expectedTime.Format(time.UnixDate),
			provider.expairedFiredIn.Format(time.UnixDate),
			diffBetween)
	}
}

type job func(int) error

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
		jobs <- func(i int) job {
			return func(n int) error {
				defer wg.Done()
				return tasks[i](n)
			}
		}(i)
	}
	cancel()
	wg.Wait()

	return
}
