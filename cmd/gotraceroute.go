package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aeden/traceroute"
)

func main() {
	maxTTL := flag.Int("m", traceroute.DefaultMaxHops, `Set the max time-to-live (max number of hops) used in outgoing probe packets (default is 64)`)
	startTTL := flag.Int("f", traceroute.DefaultStartTtl, `Set the first used time-to-live, e.g. the first hop (default is 1)`)
	retries := flag.Int("q", 1, `Set the number of probes per hop (default is one probe).`)
	jsonCompact := flag.Bool("j", false, "Out the result as a JSON in compact format (default false)")
	jsonFormatted := flag.Bool("J", false, "Out the result as a JSON in pretty format (default false)")

	flag.Parse()
	json := *jsonCompact || *jsonFormatted
	host := flag.Arg(0)
	if host == "" {
		flag.PrintDefaults()
	}
	options := traceroute.Options{}
	options.Retries = *retries - 1
	options.MaxHops = *maxTTL + 1
	options.StartTTL = *startTTL

	fmt.Println("Testing unblocking traceroute")
	c, err := traceroute.Run(context.Background(), host, options)

	if err != nil {
		fmt.Println(err)
		return
	}

	var total = 0
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
			fmt.Printf(hop.StringHuman())
		}
		total++
	}
	if total > 0 {
		if json {
			fmt.Printf("]")
		}
	}
}
