// Package traceroute provides method for executing a traceroute to a remote
// host.
// Based on traceroute implementation https://github.com/aeden/traceroute which was significantly reworked and
// as a result several annoying bugs was fixed, error handling was added, and it was adopted to concurrent execution
// Some ideas about packet construction and decoding also was get from https://github.com/Syncbak-Git/traceroute

package traceroute

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jackpal/gateway"
	"math"
	"net"
	"syscall"
	"time"
)

const DefaultPort = 33434
const DefaultMaxHops = 64
const DefaultStartTtl = 1
const DefaultTimeoutMs = 200
const DefaultRetries = 2

// Return the first non-loopback address as a 4 byte IP address. This address
// is used for sending packets out.

func findAddress() (ip net.IP, err error) {
	ip, err = gateway.DiscoverInterface()
	if err != nil {
		return localAddress()
	}

	return
}

// Return the first non-loopback address as a 4 byte IP address. This address
// is used for sending packets out.
func localAddress() (addr net.IP, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if len(ipnet.IP.To4()) == net.IPv4len {
				return ipnet.IP.To4(), nil
			}
		}
	}
	err = errors.New("you do not appear to be connected to the Internet")
	return
}

// destAddr converts a given host name to IP address
func destAddr(dest string) (destAddr net.IP, err error) {
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

// Result type
type Result struct {
	DestinationAddress net.IP
	Hops               []Hop
}

func notify(hop Hop, channels []chan Hop) {
	for _, c := range channels {
		c <- hop
	}
}

func closeNotify(channels []chan Hop) {
	for _, c := range channels {
		close(c)
	}
}

// Run uses the given dest (hostname) and options to execute a traceroute
// from your machine to the remote host.
//
// Outbound packets are UDP packets and inbound packets are ICMP.
//
// Returns a Result which contains an array of hops. Each hop includes
// the elapsed time and its IP address.
func Run(ctx context.Context, dest string, options *Options, c ...chan Hop) (result Result, err error) {

	var (
		hop          Hop
		dstAddrBytes [4]byte
		srcAddrBytes [4]byte
		port         = options.port()
	)

	socketAddr, err := findAddress()
	if err != nil {
		return
	}
	copy(srcAddrBytes[:], socketAddr.To4())

	destAddr, err := destAddr(dest)
	if err != nil {
		return
	}
	copy(dstAddrBytes[:], destAddr.To4())

	result.DestinationAddress = destAddr
	result.Hops = []Hop{}

	ttl := options.startTTL()
	packetID := 0
	payload := bytes.Repeat([]byte{0x00}, options.payloadSize())
	retry := 0

	// Set up the socket to receive inbound packets
	recvSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		return
	}
	defer func() {
		_ = syscall.Close(recvSocket)
	}()

	// Set up the socket to send packets out.
	sendSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	defer func() {
		_ = syscall.Close(sendSocket)
	}()
	if err != nil {
		return result, err
	}

	// Bind to the local socket to listen for ICMP packets
	err = syscall.Bind(recvSocket, &syscall.SockaddrInet4{Port: options.port(), Addr: srcAddrBytes})

	var packetBuff = make([]byte, 100)
	for ttl <= options.maxHops() && !hop.Node.IP.Equal(destAddr) {
		start := time.Now()
		packetID = (packetID + 1) % math.MaxUint16

		pkt := newUDPPacket(destAddr, port, port, ttl, packetID, payload)
		flowId := ipFlowID(packetID, destAddr, port, port)
		// Send a UDP packet
		e := syscall.Sendto(sendSocket, pkt, 0, &syscall.SockaddrInet4{Port: port, Addr: dstAddrBytes})
		if e != nil {
			err = fmt.Errorf("sendto error: %w", e)
			break
		}

		done := false
		timeout := options.timeout()
		// the socket receives any ICMP packets from anyone,
		// so we need to filter and drop packets anyone else's ICMP packets and continue to receive
		// with reduced timeout till the overall timeout happened or our target packet received
		for !done && timeout > 0 {
			// solution with SetsockoptTimeval isn't optimal, it's better to use poll (epoll)
			tv := syscall.NsecToTimeval(timeout.Nanoseconds())
			// This sets the timeout to wait for a response from the remote host
			if err = syscall.SetsockoptTimeval(recvSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
				return
			}
			_, _, err := syscall.Recvfrom(recvSocket, packetBuff, 0)
			now := time.Now()
			elapsed := now.Sub(start)

			if err == nil {
				hop, e = extractMessage(packetBuff)
				if e != nil || hop.ID != flowId {
					timeout = timeout - elapsed
					continue
				}

				done = true

				hop.Step = ttl
				hop.Sent = start
				hop.Received = now
				hop.Elapsed = elapsed
			} else {
				// timeout
				if err == syscall.EWOULDBLOCK {
					timeout = 0
				} else {
					// something bad
					time.Sleep(time.Millisecond * 10)
					timeout = timeout - time.Since(start)
				}
			}
		}

		if done {
			result.Hops = append(result.Hops, hop)
		} else {
			retry++
			if retry <= options.retries() {
				continue
			}
			hop = newHop(packetID, socketAddr, destAddr, port, port)
			hop.Step = ttl
			hop.Sent = start
			hop.Elapsed = time.Since(start)
		}

		notify(hop, c)
		ttl += 1
		retry = 0
	}
	closeNotify(c)
	return
}
