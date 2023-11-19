// Package traceroute provides method for executing a traceroute to a remote
// host.
// Based on traceroute implementation https://github.com/aeden/traceroute which was significantly reworked and
// as a result several annoying bugs was fixed, error handling was added, and it was adopted to concurrent execution
// Some ideas about packet construction and decoding also was get from https://github.com/Syncbak-Git/traceroute

package traceroute

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
)

const DefaultPort = 33434
const DefaultMaxHops = 64
const DefaultStartTtl = 1
const DefaultTimeoutMs = 200
const DefaultRetries = 2

// destIp converts a given host name to IP address
func destIp(dest string) (destAddr net.IP, err error) {
	addrs, err := net.LookupHost(dest)
	if err != nil {
		return
	}
	addr := addrs[0]
	ipAddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		return
	}
	return ipAddr.IP, nil
}

// Run uses the given dest (hostname) and options to execute a traceroute
// to the remote host.
// Run is unblocked and returns a communication channel where the caller should read the Hop data
// On finish or error the communication channel will be closed
// Outbound packets are UDP packets and inbound packets are ICMP.
func Run(ctx context.Context, dest string, options Options) (c chan Hop, err error) {

	destAddr, err := destIp(dest)
	if err != nil {
		return
	}

	flow, err := newFlow(destAddr, options.Port)
	if err != nil {
		return
	}

	c = make(chan Hop)
	go func() {
		_, err = run(ctx, options, flow, c)
		flow.close()
		close(c)
	}()

	return
}

// RunBlock uses the given dest (hostname) and options to execute a traceroute
// to the remote host.
// RunBlock is blocked until traceroute finished and returns a Result which contains an array of hops. Each hop includes
// the elapsed time and its IP address.
// Outbound packets are UDP packets and inbound packets are ICMP.
func RunBlock(dest string, options Options) (hops []Hop, err error) {
	destAddr, err := destIp(dest)
	if err != nil {
		return
	}

	flow, err := newFlow(destAddr, options.Port)
	if err != nil {
		return
	}

	hops, err = run(context.Background(), options, flow, nil)

	flow.close()

	return
}

func run(ctx context.Context, options Options, f flow, c chan<- Hop) (hops []Hop, err error) {
	var hop Hop
	port := options.port()

	var dstAddrBytes [4]byte
	copy(dstAddrBytes[:], f.destAddr.To4())

	ttl := options.startTTL()

	var packetIdx uint16
	payload := bytes.Repeat([]byte{0x00}, options.payloadSize())
	retry := 0

	var recvBuff = make([]byte, 100)
	for ttl <= options.maxHops() && !hop.Node.IP.Equal(f.destAddr) {
		start := time.Now()
		packetIdx = (packetIdx + 1) % (1<<6 - 1)
		packetID := int(f.flowId<<6 + packetIdx)
		//fmt.Printf("send packetIdx: %v, packetID: %v, ttl: %v\n", packetIdx, packetID, ttl)
		pkt := newUDPPacket(f.destAddr, port, port, ttl, packetID, payload)
		//flowId := ipFlowID(f.flowId, f.destAddr, port, port)
		// Send a UDP packet
		e := syscall.Sendto(f.sSocket, pkt, 0, &syscall.SockaddrInet4{Port: port, Addr: dstAddrBytes})
		if e != nil {
			err = fmt.Errorf("sendto error: %w", e)
			break
		}

		timeout := options.timeout()
		// in general the socket can receive any ICMP packets from anyone,
		// so we need to filter and drop anyone else's ICMP packets and continue to receive
		// with reduced timeout till the overall timeout happened or our target packet received
		//
		// It makes no sense if we use BPF filter, but we leave this solution here for a general case,
		// if bpf filter disabled or not supported by OS, this solution guarantees a correct reception at least for single-threaded traceroute
		for timeout > 0 {
			// solution with SetsockoptTimeval isn't optimal, it's better to use poll (epoll)
			tv := syscall.NsecToTimeval(timeout.Nanoseconds())
			// This sets the timeout to wait for a response from the remote host
			if err = syscall.SetsockoptTimeval(f.rSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
				return
			}
			_, _, err := syscall.Recvfrom(f.rSocket, recvBuff, 0)
			now := time.Now()
			elapsed := now.Sub(start)

			if err == nil {
				hop, e = extractMessage(recvBuff)
				//fmt.Printf("recv ID: %v\n", hop.ID)
				//fmt.Println(hop.StringHuman())
				if e != nil || hop.ID != packetID {
					timeout = timeout - elapsed
					continue
				}

				hop.Success = true
				hop.Step = ttl
				hop.Sent = start
				hop.Received = now
				hop.Elapsed = elapsed
				break
			} else {
				// timeout
				if err == syscall.EWOULDBLOCK {
					timeout = 0
				} else {
					// something bad (lack of resources or something else)
					time.Sleep(time.Millisecond * 10)
					timeout = timeout - time.Since(start)
				}
			}
		}

		if timeout <= 0 {
			retry++
			if retry <= options.retries() {
				continue
			}
			hop = newHop(int(f.flowId), f.socketAddr, f.destAddr, ttl)
			hop.Sent = start
			hop.Elapsed = time.Since(start)
		}

		hops = append(hops, hop)
		if c != nil {
			c <- hop
		}
		ttl += 1
		retry = 0
	}
	return
}
