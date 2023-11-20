package gotraceroute

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
)

var nextFlowId uint16
var nextFlowIDMutex sync.Mutex

// flow describes one traceroute flow to the address destAddr,
// should be created with newFlow and close() method should be call when
// traceroute is finished
type flow struct {
	socketAddr net.IP
	destAddr   net.IP
	sSocket    int
	rSocket    int
	flowID     uint16
}

func (f *flow) close() {
	_ = syscall.Close(f.sSocket)
	_ = syscall.Close(f.rSocket)
}

// newFlow initializes sockets and returns flow struct
//
//nolint:funlen
func newFlow(destAddr net.IP, srcPort int, networkInterface string) (f flow, err error) {
	f.destAddr = destAddr
	f.socketAddr, err = findSocketAddress(networkInterface)
	if err != nil {
		return
	}

	addr := syscall.SockaddrInet4{
		Port: srcPort,
	}
	copy(addr.Addr[:], f.socketAddr.To4())

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

	// Bind the socket to network addr to listen for ICMP packets
	err = syscall.Bind(f.rSocket, &addr)
	if err != nil {
		err = fmt.Errorf("can't bind recv socket: %w", err)
		return
	}

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

	// Bind sending socket
	//
	// thi is disabled, cause there are some side effects with advanced routing scenarios
	// when output interface is set to a not default gateway interface
	// in this case the packet is sending by the kernel through default gateway anyway
	// but the packet has sending ip address of not a gateway interface but configured interface
	// workaround is needed
	/*
		err = syscall.Bind(f.sSocket, &addr)
		if err != nil {
			err = fmt.Errorf("can't bind send socket: %w", err)
			return
		}
	*/

	// assign flowId to identify only this flow packets on a raw socket
	nextFlowIDMutex.Lock()
	f.flowID = nextFlowId
	nextFlowId = (nextFlowId + 1) % (1<<10 - 1) // max 10 bits assign to flowId
	nextFlowIDMutex.Unlock()

	// apply BPF filter to the socket
	err = bpfFlowID(f.flowID).applyToSocket(f.rSocket)

	if err != nil {
		err = fmt.Errorf("can't apply bpf filter: %w", err)
	}
	return
}

// findSocketAddress returns the address is used for sending packets out.
// networkInterface contains the name of interface used for sending packets out.
// if networkInterface is empty the default gateway interface is used
// or if there is problem to invoke gateway interface,
// the first non-loopback address is used
func findSocketAddress(networkInterface string) (ip net.IP, err error) {
	if networkInterface == "" {
		ip = net.ParseIP("0.0.0.0").To4()
		// disabled as we relly on the network stack in choosing output interface
		// ip, err = gateway.DiscoverInterface()
	}
	if err != nil || networkInterface != "" {
		return localAddress(networkInterface)
	}

	return
}

// localAddress returns the first non-loopback address as a 4 byte IP address.
// This address is used for sending packets out.
func localAddress(networkInterface string) (addr net.IP, err error) {
	var (
		ifc   *net.Interface
		addrs []net.Addr
	)
	if networkInterface == "" {
		if addrs, err = net.InterfaceAddrs(); err != nil {
			return
		}
	} else {
		if ifc, err = net.InterfaceByName(networkInterface); err != nil {
			return
		}
		if addrs, err = ifc.Addrs(); err != nil {
			return
		}
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
