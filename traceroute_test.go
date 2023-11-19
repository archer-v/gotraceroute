package traceroute

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

var testHosts = []string{"google.com", "starshiptroopers.dev", "8.8.8.8", "1.1.1.1", "yahoo.com"}

// var testHosts = []string{"google.com", "facebook.com", "starshiptroopers.dev", "msn.com", "bing.com", "8.8.8.8"}
var testErrHosts = []string{"aaaabbbcczzzzzz", "aaaabbbcczzzzzzdddd1.com", "266.266.266.266"}

func TestRunBlock(t *testing.T) {
	fmt.Println("Testing blocking traceroute...")

	hops, err := RunBlock(testHosts[0], Options{})
	if err == nil {
		if len(hops) == 0 {
			t.Errorf("TestTraceroute failed. Expected at least one hop")
		}
	} else {
		t.Errorf("TestTraceroute failed due to an error: %v", err)
	}

	for _, hop := range hops {
		fmt.Println(hop.StringHuman())
	}
	fmt.Println()
}

func TestRun(t *testing.T) {
	fmt.Println("Testing unblocking traceroute")
	c, err := Run(context.Background(), testHosts[0], Options{})

	if err != nil {
		t.Errorf("TestTraceroute failed due to an error: %v", err)
		return
	}
	for hop := range c {
		fmt.Println(hop.StringHuman())
	}
}

func TestRun3Error(t *testing.T) {
	for _, h := range testErrHosts {
		_, err := RunBlock(h, Options{})
		if err == nil {
			t.Errorf("TestTraceroute failed. Expected error on host: %v", h)
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
		//results[i] = make([]Hop, options.MaxHops)
		results[i] = []Hop{}
		var mu sync.Mutex

		wg.Add(1)
		go func(i int, host string, channel chan Hop) {
			for hop := range channel {
				//fmt.Printf("Traceroute %v: %v\n", i, hop.StringHuman())
				mu.Lock()
				results[i] = append(results[i], hop)
				//results[i][hop.Step-1] = hop
				mu.Unlock()
			}
			fmt.Printf("Traceroute %v to %v: finished\n", i+1, host)
			wg.Done()
		}(i, h, channels[i])
	}

	wg.Wait()

	for i, r := range results {
		fmt.Printf("Traceroute result %v to host %v:\n", i+1, testHosts[i])
		for _, h := range r {
			if h.Step > 0 {
				fmt.Println(h.StringHuman())
			}
		}
	}
}
