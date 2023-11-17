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

// Result type
type Result struct {
	DestinationAddress net.IP
	Hops               []Hop
}

// Run uses the given dest (hostname) and options to execute a traceroute
// to the remote host.
// Run is unblocked and returns a communication channel where the caller should read the Hop data
// On finish or error the communication channel will be closed
// Outbound packets are UDP packets and inbound packets are ICMP.
func Run(ctx context.Context, dest string, options Options) (c chan Hop, err error) {
	c = make(chan Hop)

	socketAddr, destAddr, sSocket, rSocket, err := prepare(dest, options)
	if err != nil {
		return
	}

	go func() {
		_, err = run(context.Background(), options, socketAddr, destAddr, sSocket, rSocket, c)
		_ = syscall.Close(sSocket)
		_ = syscall.Close(rSocket)
		close(c)
	}()

	return
}

// RunBlock uses the given dest (hostname) and options to execute a traceroute
// to the remote host.
// RunBlock is blocked until traceroute finished and returns a Result which contains an array of hops. Each hop includes
// the elapsed time and its IP address.
// Outbound packets are UDP packets and inbound packets are ICMP.
func RunBlock(dest string, options Options) (result Result, err error) {
	socketAddr, destAddr, sSocket, rSocket, err := prepare(dest, options)
	if err != nil {
		return
	}

	result, err = run(context.Background(), options, socketAddr, destAddr, sSocket, rSocket, nil)

	_ = syscall.Close(sSocket)
	_ = syscall.Close(rSocket)

	return
}

func prepare(dest string, options Options) (socketAddr net.IP, destAddr net.IP, sSocket int, rSocket int, err error) {
	var (
		srcAddrBytes [4]byte
	)

	socketAddr, err = findAddress()
	if err != nil {
		return
	}
	copy(srcAddrBytes[:], socketAddr.To4())

	destAddr, err = destIp(dest)
	if err != nil {
		return
	}

	// Set up the socket to receive inbound packets
	rSocket, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = syscall.Close(rSocket)
		}
	}()

	// Set up the socket to send packets out.
	sSocket, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = syscall.Close(sSocket)
		}
	}()

	// Bind to the local socket to listen for ICMP packets
	err = syscall.Bind(rSocket, &syscall.SockaddrInet4{Port: options.port(), Addr: srcAddrBytes})
	return
}

func run(ctx context.Context, options Options, socketAddr net.IP, destAddr net.IP, sSocket int, rSocket int, c chan Hop) (result Result, err error) {
	var hop Hop
	port := options.port()

	var dstAddrBytes [4]byte
	copy(dstAddrBytes[:], destAddr.To4())

	result.DestinationAddress = destAddr
	result.Hops = []Hop{}

	ttl := options.startTTL()
	packetID := 0
	payload := bytes.Repeat([]byte{0x00}, options.payloadSize())
	retry := 0

	var recvBuff = make([]byte, 100)
	for ttl <= options.maxHops() && !hop.Node.IP.Equal(destAddr) {
		start := time.Now()
		packetID = (packetID + 1) % math.MaxUint16

		pkt := newUDPPacket(destAddr, port, port, ttl, packetID, payload)
		flowId := ipFlowID(packetID, destAddr, port, port)
		// Send a UDP packet
		e := syscall.Sendto(sSocket, pkt, 0, &syscall.SockaddrInet4{Port: port, Addr: dstAddrBytes})
		if e != nil {
			err = fmt.Errorf("sendto error: %w", e)
			break
		}

		timeout := options.timeout()
		// the socket receives any ICMP packets from anyone,
		// so we need to filter and drop anyone else's ICMP packets and continue to receive
		// with reduced timeout till the overall timeout happened or our target packet received
		for timeout > 0 {
			// solution with SetsockoptTimeval isn't optimal, it's better to use poll (epoll)
			tv := syscall.NsecToTimeval(timeout.Nanoseconds())
			// This sets the timeout to wait for a response from the remote host
			if err = syscall.SetsockoptTimeval(rSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
				return
			}
			_, _, err := syscall.Recvfrom(rSocket, recvBuff, 0)
			now := time.Now()
			elapsed := now.Sub(start)

			if err == nil {
				hop, e = extractMessage(recvBuff)
				if e != nil || hop.ID != flowId {
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
			hop = newHop(packetID, socketAddr, destAddr, port, port)
			hop.Step = ttl
			hop.Sent = start
			hop.Elapsed = time.Since(start)
		}

		result.Hops = append(result.Hops, hop)
		if c != nil {
			c <- hop
		}
		ttl += 1
		retry = 0
	}
	return
}
