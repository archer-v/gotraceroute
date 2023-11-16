package traceroute

import (
	"fmt"
	"net"
	"time"
)

// Addr is a network address stored as a pair of domain name and ip address
type Addr struct {
	// Host is the host (ie, DNS) name of the node.
	Host string
	// IP is the IP address of the node.
	IP net.IP
}

func (a *Addr) String() string {
	if a.Host != "" {
		return fmt.Sprintf("%s (%s)", a.Host, a.IP.String())
	}
	return a.IP.String()
}

func (a *Addr) HostOrAddr() string {
	if a.Host != "" {
		return a.Host
	}
	return a.IP.String()
}

// Hop is a step in the network route between a source and destination address.
type Hop struct {
	// Src is the source (ie, local) address.
	Src Addr
	// Dst is the destination (ie, remote) address.
	Dst Addr
	// Node is the node at this step of the route.
	Node Addr
	// Step is the location of this node in the route, ie the TTL value used.
	Step int
	// ID is a unique ID that is used to match the original request with the ICMP response.
	// It can be derived from either the request or the response.
	ID string
	// DstPort is the destination port targeted.
	DstPort int
	// Sent is the time the query began.
	Sent time.Time
	// Received is the time the query completed.
	Received time.Time
	// Elapsed is the duration of the query.
	Elapsed time.Duration
	// IcmpType is the received ICMP packet type value.
	IcmpType int
}

func (h *Hop) String() string {
	return fmt.Sprintf("Src: %s, Dst: %s (%s), Node: %s (%s), Step: %d, Elapsed: %s, ID: %s, Type: %d",
		h.Src.IP.String(), h.Dst.Host, h.Dst.IP.String(), h.Node.Host, h.Node.IP.String(), h.Step, h.Elapsed.String(), h.ID, h.IcmpType)
}

func (h *Hop) Fields() map[string]interface{} {
	return map[string]interface{}{
		"srchost":  h.Src.Host,
		"srcip":    h.Src.IP.String(),
		"dsthost":  h.Dst.Host,
		"dstip":    h.Dst.IP.String(),
		"nodehost": h.Node.Host,
		"nodeip":   h.Node.IP.String(),
		"step":     h.Step,
		"id":       h.ID,
		"sent":     h.Sent.Format(time.RFC3339Nano),
		"received": h.Received.Format(time.RFC3339Nano),
		"elapsed":  h.Elapsed.Seconds(),
		"ms":       h.Elapsed.Milliseconds(),
	}
}

func newHop(flowId int, src net.IP, dst net.IP, sport int, dport int) Hop {
	return Hop{
		Src: Addr{
			IP: src,
		},
		Dst: Addr{
			IP: dst,
		},
		ID: ipFlowID(flowId, dst, sport, dport),
	}
}