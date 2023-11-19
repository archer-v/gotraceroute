package traceroute

import (
	"errors"
	"fmt"
	"github.com/jackpal/gateway"
	"net"
	"sync"
	"syscall"
)

var nextFlowId uint16 = 0
var nextFlowIdMutex sync.Mutex

// flow describes one traceroute flow to the address destAddr,
// should be created with newFlow and close() method should be call when
// traceroute is finished
type flow struct {
	socketAddr net.IP
	destAddr   net.IP
	sSocket    int
	rSocket    int
	flowId     uint16
}

func (f *flow) close() {
	_ = syscall.Close(f.sSocket)
	_ = syscall.Close(f.rSocket)
}

// newFlow initializes sockets and returns flow struct
func newFlow(destAddr net.IP, srcPort int) (f flow, err error) {
	var (
		srcAddrBytes [4]byte
	)
	f.destAddr = destAddr
	f.socketAddr, err = findAddress()
	if err != nil {
		return
	}
	copy(srcAddrBytes[:], f.socketAddr.To4())

	// Set up the socket to receive inbound packets
	f.rSocket, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		err = fmt.Errorf("can't create a recv socket: %w", err)
		return
	}
	defer func() {
		if err != nil {
			_ = syscall.Close(f.rSocket)
		}
	}()

	// Set up the socket to send packets out.
	f.sSocket, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		err = fmt.Errorf("can't create a send socket: %w", err)
		return
	}
	defer func() {
		if err != nil {
			_ = syscall.Close(f.sSocket)
		}
	}()

	// Bind to the local socket to listen for ICMP packets
	err = syscall.Bind(f.rSocket, &syscall.SockaddrInet4{Port: srcPort, Addr: srcAddrBytes})
	if err != nil {
		err = fmt.Errorf("can't bind recv socket: %w", err)
		return
	}

	// assign flowId to identify only this flow packets on a raw socket
	nextFlowIdMutex.Lock()
	f.flowId = nextFlowId
	nextFlowId = (nextFlowId + 1) % (1<<10 - 1) // max 10 bits assign to flowId
	nextFlowIdMutex.Unlock()

	// apply BPF filter to the socket
	err = bpfFlowId(f.flowId).applyToSocket(f.rSocket)

	if err != nil {
		err = fmt.Errorf("can't apply bpf filter: %w", err)
	}
	return
}

// findAddress returns the first non-loopback address as a 4 byte IP address. This address
// is used for sending packets out.
func findAddress() (ip net.IP, err error) {
	ip, err = gateway.DiscoverInterface()
	if err != nil {
		return localAddress()
	}

	return
}

// localAddress returns the first non-loopback address as a 4 byte IP address.
// This address is used for sending packets out.
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
