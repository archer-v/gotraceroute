package traceroute

// borrowed from https://github.com/Syncbak-Git/traceroute/blob/master/socket.go

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"math"
	"net"
	"syscall"
)

type udpHeader struct {
	SourcePort uint16
	DestPort   uint16
	Length     uint16
	Checksum   uint16
}

func newUDPPacket(dst net.IP, srcPort, dstPort int, ttl, id int, payload []byte) []byte {
	ipHeader := ipv4.Header{
		Version:  ipv4.Version,          // protocol version
		Len:      ipv4.HeaderLen,        // header length
		TotalLen: 20 + 8 + len(payload), // packet total length: 20 IP, 8 UDP
		ID:       id % math.MaxUint16,   // identification
		TTL:      ttl,                   // time-to-live
		Protocol: syscall.IPPROTO_UDP,   // next protocol
		Dst:      dst,                   // destination address
		// the other fields, including Src, will be filled in by the kernel
	}
	udp := udpHeader{
		SourcePort: uint16(srcPort % math.MaxUint16),
		DestPort:   uint16(dstPort % math.MaxUint16),
		Length:     uint16(8 + len(payload)),
		// We'll leave checksum empty. It's optional in ipv4, and maybe the kernel will calculate it for us
	}
	b, _ := ipHeader.Marshal()
	data := bytes.NewBuffer(b)
	_ = binary.Write(data, binary.BigEndian, udp)
	data.Write(payload)
	return data.Bytes()
}

func extractMessage(p []byte, resolveToName bool) (hop Hop, err error) {
	// borrowed from https://github.com/Syncbak-Git/traceroute/blob/master/icmp.go

	// get the reply IPv4 header. That will have the node address
	replyHeader, err := icmp.ParseIPv4Header(p)
	if err != nil {
		return
	}
	// now, extract the ICMP message
	msg, err := icmp.ParseMessage(syscall.IPPROTO_ICMP, p[replyHeader.Len:])
	if err != nil {
		return
	}
	icmpType := int(p[replyHeader.Len])
	var data []byte
	if te, ok := msg.Body.(*icmp.TimeExceeded); ok {
		data = te.Data
	} else if du, ok := msg.Body.(*icmp.DstUnreach); ok {
		data = du.Data
	} else {
		return
	}
	// data should now have the IP header of the original message plus at least
	// 8 bytes of the original message (which is, at least, the UDP header)
	srcHeader, err := icmp.ParseIPv4Header(data)
	if err != nil {
		return
	}
	udpHeader := data[srcHeader.Len:]
	if len(udpHeader) < 8 {
		err = fmt.Errorf("source udp header too short: %d", len(udpHeader))
		return
	}
	//srcPort := binary.BigEndian.Uint16(udpHeader[0:2])
	dstPort := binary.BigEndian.Uint16(udpHeader[2:4])

	hop = newHop(srcHeader.ID, srcHeader.Src, srcHeader.Dst, srcHeader.TTL)
	hop.IcmpType = icmpType
	hop.DstPort = int(dstPort)
	hop.Node = Addr{
		IP: replyHeader.Src,
	}

	if resolveToName {
		names, _ := net.LookupAddr(replyHeader.Src.String())
		if len(names) > 0 {
			hop.Node.Host = names[0]
		}
	}
	return
}

func ipFlowID(ipHeaderID int, destIP net.IP, srcPort, destPort int) string {
	return fmt.Sprintf("%d|%s|%d|%d", ipHeaderID, destIP.String(), srcPort, destPort)
}
