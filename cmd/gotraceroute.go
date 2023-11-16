package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aeden/traceroute"
	"net"
)

func printHop(hop traceroute.Hop) {
	if hop.IcmpType != 0 {
		fmt.Printf("%-3d %v (%v)  %vms\n", hop.Step, hop.Node.HostOrAddr(), hop.Node.IP.String(), hop.Elapsed.Milliseconds())
	} else {
		fmt.Printf("%-3d *\n", hop.Step)
	}
}

func address(address [4]byte) string {
	return fmt.Sprintf("%v.%v.%v.%v", address[0], address[1], address[2], address[3])
}

func main() {
	var m = flag.Int("m", traceroute.DefaultMaxHops, `Set the max time-to-live (max number of hops) used in outgoing probe packets (default is 64)`)
	var f = flag.Int("f", traceroute.DefaultStartTtl, `Set the first used time-to-live, e.g. the first hop (default is 1)`)
	var q = flag.Int("q", 1, `Set the number of probes per "ttl" to nqueries (default is one probe).`)

	flag.Parse()
	host := flag.Arg(0)
	options := traceroute.Options{}
	options.Retries = *q - 1
	options.MaxHops = *m + 1
	options.StartTTL = *f

	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return
	}

	fmt.Printf("traceroute to %v (%v), %v hops max, %v byte packet payload\n", host, ipAddr, options.MaxHops, options.PayloadSize)

	c := make(chan traceroute.Hop, 0)
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

	ctx := context.Background()
	_, err = traceroute.Run(ctx, host, &options, c)
	if err != nil {
		fmt.Printf("Error: ", err)
	}
}
