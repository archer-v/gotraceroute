package traceroute

import (
	"context"
	"fmt"
	"testing"
)

var testHosts = []string{"google.com", "facebook.com", "starshiptroopers.dev", "msn.com", "bing.com", "8.8.8.8"}
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

/*
func TestRun4Concurrent(t *testing.T) {
	chans := make([]chan Hop, len(testHosts))

	for i, h := range testHosts {
		chans[i] = make(chan Hop)
		out, err := Run(context.Background(), testHosts[0], Options{}, chans[i])
	}
}


*/
