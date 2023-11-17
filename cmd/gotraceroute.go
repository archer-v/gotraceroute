package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aeden/traceroute"
	"os"
	"time"
)

func main() {
	maxTTL := flag.Int("m", traceroute.DefaultMaxHops, `Set the max time-to-live (max number of hops) used in outgoing probe packets`)
	startTTL := flag.Int("f", traceroute.DefaultStartTtl, `Set the first used time-to-live, e.g. the first hop`)
	retries := flag.Int("q", 1, `Set the number of probes per hop`)
	port := flag.Int("p", traceroute.DefaultPort, `Set source and destination port to use`)
	timeout := flag.Duration("z", time.Millisecond*traceroute.DefaultTimeoutMs, "Waiting timeout in ms")
	pSize := flag.Int("l", 0, `Packet length`)
	jsonCompact := flag.Bool("j", false, "Out the result as a JSON in compact format")
	jsonFormatted := flag.Bool("J", false, "Out the result as a JSON in pretty format")

	flag.Parse()
	json := *jsonCompact || *jsonFormatted
	host := flag.Arg(0)
	if host == "" {
		flag.Usage()
		os.Exit(1)
	}
	options := traceroute.Options{
		Retries:     *retries,
		MaxHops:     *maxTTL,
		StartTTL:    *startTTL,
		Port:        *port,
		Timeout:     *timeout,
		PayloadSize: *pSize,
	}

	c, err := traceroute.Run(context.Background(), host, options)

	if err != nil {
		fmt.Println(err)
		return
	}

	var lastHop traceroute.Hop
	for hop := range c {
		if hop.Step == 1 {
			if json {
				fmt.Printf("[")
			} else {
				fmt.Printf("traceroute to %v (%v), %v hops max, %v byte packet payload\n", host, hop.Dst.IP.String(), options.MaxHops, options.PayloadSize)
			}
		} else {
			if json {
				fmt.Printf(",")
				if *jsonFormatted {
					fmt.Println()
				}
			}
		}
		if json {
			fmt.Printf(hop.StringJSON(*jsonFormatted))
		} else {
			fmt.Println(hop.StringHuman())
		}
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
