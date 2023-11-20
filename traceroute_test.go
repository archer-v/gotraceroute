package traceroute

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

var testHosts = []string{"google.com", "starshiptroopers.dev", "8.8.8.8", "1.1.1.1", "yahoo.com"}

var testErrHosts = []string{"aaaabbbcczzzzzz", "aaaabbbcczzzzzzdddd1.com", "266.266.266.266"}

func TestRunBlock(t *testing.T) {
	fmt.Println("Testing blocking traceroute...")

	hops, err := RunBlock(testHosts[0], Options{})
	if err == nil {
		if len(hops) == 0 {
			t.Errorf("TestRunBlock failed. Expected at least one hop")
		}
	} else {
		t.Errorf("TestRunBlock failed due to an error: %v", err)
	}

	for _, hop := range hops {
		fmt.Println(hop.StringHuman())
	}
	fmt.Println()
}

func TestRun1(t *testing.T) {
	fmt.Println("Testing unblocking traceroute")
	hops, err := testRun(context.Background(), testHosts[0], Options{})

	if err != nil {
		t.Errorf("TestRun1 failed due to an error: %v", err)
		return
	}
	if len(hops) == 0 {
		t.Errorf("TestRun1 failed. Expected at least one hop")
	}
}

func TestRun2WithDeadline(t *testing.T) {
	fmt.Println("Testing unblocking traceroute with deadline")
	timeout := time.Millisecond * 400
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	started := time.Now()
	hops, err := testRun(ctx, testHosts[0], Options{})

	if err != nil {
		t.Errorf("TestRun2WithDeadline failed due to an error: %v", err)
		return
	}
	if len(hops) == 0 {
		t.Errorf("TestRun2WithDeadline failed. Expected at least one hop")
	}

	if hops[len(hops)-1].Received.Sub(started) > timeout {
		t.Errorf("TestRun2WithDeadline failed. Should stop not later than %v", timeout)
	}
}

func TestRun3Error(t *testing.T) {
	for _, h := range testErrHosts {
		_, err := RunBlock(h, Options{})
		if err == nil {
			t.Errorf("TestRun3Error failed. Expected error on host: %v", h)
		} else {
			fmt.Printf("Got error as expected on traceroute to host %v: %v\n", h, err)
		}
	}
}

func TestRun4Concurrent(t *testing.T) {
	channels := make([]chan Hop, len(testHosts))
	results := make([][]Hop, len(testHosts))
	var err error
	ctx := context.Background()
	options := Options{
		MaxHops: 32,
	}

	var wg sync.WaitGroup

	for i, h := range testHosts {
		channels[i], err = Run(ctx, h, options)
		var status string
		if err != nil {
			status = err.Error()
		} else {
			status = "started"
		}
		fmt.Printf("Traceroute %v to %v: %v\n", i+1, h, status)

		if err != nil {
			continue
		}

		results[i] = []Hop{}
		var mu sync.Mutex

		wg.Add(1)
		go func(i int, host string, channel chan Hop) {
			for hop := range channel {
				mu.Lock()
				results[i] = append(results[i], hop)
				mu.Unlock()
			}
			fmt.Printf("Traceroute %v to %v: finished\n", i+1, host)
			wg.Done()
		}(i, h, channels[i])
	}

	wg.Wait()

	for i, r := range results {
		fmt.Printf("Traceroute result %v to host %v:\n", i+1, testHosts[i])
		succCount := 0
		for _, h := range r {
			if h.Step > 0 {
				fmt.Println(h.StringHuman())
			}
			if h.Success {
				succCount++
			}
		}
		if succCount == 0 {
			t.Errorf("Traceroute result to %v doesn't contain success hops", testHosts[i])
		}
	}
}

func testRun(ctx context.Context, host string, options Options) (hops []Hop, err error) {
	c, err := Run(ctx, host, options)

	if err != nil {
		return
	}
	for hop := range c {
		hops = append(hops, hop)
		fmt.Println(hop.StringHuman())
	}
	return
}

func testCountSuccess(hops []Hop) (n int) {
	for _, h := range hops {
		if h.Success {
			n++
		}
	}
	return
}
