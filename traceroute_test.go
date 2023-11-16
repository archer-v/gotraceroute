package traceroute

import (
	"context"
	"fmt"
	"testing"
)

func TestTraceroute(t *testing.T) {
	fmt.Println("Testing synchronous traceroute")

	out, err := Run(context.Background(), "google.com", new(Options))
	if err == nil {
		if len(out.Hops) == 0 {
			t.Errorf("TestTraceroute failed. Expected at least one hop")
		}
	} else {
		t.Errorf("TestTraceroute failed due to an error: %v", err)
	}

	for _, hop := range out.Hops {
		printHop(hop)
	}
	fmt.Println()
}

func TestTraceouteChannel(t *testing.T) {
	fmt.Println("Testing asynchronous traceroute")
	c := make(chan Hop, 0)
	go func() {
		for {
			hop, ok := <-c
			if !ok {
				fmt.Println()
				return
			}
			printHop(hop)
		}
	}()

	out, err := Run(context.Background(), "google.com", new(Options), c)
	if err == nil {
		if len(out.Hops) == 0 {
			t.Errorf("TestTracerouteChannel failed. Expected at least one hop")
		}
	} else {
		t.Errorf("TestTraceroute failed due to an error: %v", err)
	}
}

func printHop(hop Hop) {
	//fmt.Println(hop.String())
	fmt.Printf("%-3d %v (%v)  %v\n", hop.Step, hop.Node.HostOrAddr(), hop.Node.IP.String(), hop.Elapsed)
}
