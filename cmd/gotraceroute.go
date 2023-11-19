package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/archer-v/gotraceroute"
	"os"
	"time"
)

var (
	options       traceroute.Options
	json          bool
	jsonCompact   bool
	jsonFormatted bool
	host          string
)

func main() {
	flag.IntVar(&options.MaxHops, "m", traceroute.DefaultMaxHops, `Set the max time-to-live (max number of hops) used in outgoing probe packets`)
	flag.IntVar(&options.StartTTL, "f", traceroute.DefaultStartTtl, `Set the first used time-to-live, e.g. the first hop`)
	flag.IntVar(&options.Retries, "q", 1, `Set the number of probes per hop`)
	flag.IntVar(&options.Port, "p", traceroute.DefaultPort, `Set source and destination port to use`)
	flag.DurationVar(&options.Timeout, "z", time.Millisecond*traceroute.DefaultTimeoutMs, "Waiting timeout in ms")
	flag.IntVar(&options.PayloadSize, "l", 0, `Packet length`)
	flag.BoolVar(&jsonCompact, "j", false, "Out the result as a JSON in compact format")
	flag.BoolVar(&jsonFormatted, "J", false, "Out the result as a JSON in pretty format")

	flag.Parse()
	json = jsonCompact || jsonFormatted
	host = flag.Arg(0)
	if host == "" {
		flag.Usage()
		os.Exit(1)
	}

	c, err := traceroute.Run(context.Background(), host, options)

	if err != nil {
		fmt.Println(err)
		return
	}

	var lastHop traceroute.Hop
	for hop := range c {
		displayHop(hop)
		lastHop = hop
	}

	if lastHop.Step != 0 {
		if json {
			fmt.Printf("]")
		}
		if lastHop.Success && lastHop.Node.IP.Equal(lastHop.Dst.IP) {
			os.Exit(0)
		}
	}

	os.Exit(2)
}

func displayHop(h traceroute.Hop) {
	if h.Step == 1 {
		if json {
			fmt.Printf("[")
		} else {
			fmt.Printf("traceroute to %v (%v), %v hops max, %v byte packet payload\n", host, h.Dst.IP.String(), options.MaxHops, options.PayloadSize)
		}
	} else {
		if json {
			fmt.Printf(",")
			if jsonFormatted {
				fmt.Println()
			}
		}
	}
	if json {
		fmt.Printf(h.StringJSON(jsonFormatted))
	} else {
		fmt.Println(h.StringHuman())
	}
}
