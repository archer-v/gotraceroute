package traceroute

import (
	"fmt"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"syscall"
	"unsafe"
)

type BPF []bpf.Instruction

// bpfFlowId returns a bfp program instructions that filters a traffic by flowId
func bpfFlowID(flowId uint16) BPF {
	return []bpf.Instruction{
		// Load Protocol field of IP header
		bpf.LoadAbsolute{Off: 0x09, Size: 1},
		// Skip over the next instruction if payload is ICMP.
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: syscall.IPPROTO_ICMP, SkipTrue: 1},
		// return
		bpf.RetConstant{Val: 0},
		// Load ID field of cloned source IP header from ICMP packet:
		// 28 bytes is an offset of data field in ICMP packet + 4 bytes
		bpf.LoadAbsolute{Off: 28 + 4, Size: 2},
		// apply mask of flowId field
		bpf.ALUOpConstant{Op: bpf.ALUOpAnd, Val: (1<<10 - 1) << 6},
		// Skip over the next instruction if packet id isn't flowId .
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(flowId << 6), SkipTrue: 1},
		// return
		bpf.RetConstant{Val: 0},
		// Verdict is "send up to 256bytes of the packet to userspace."
		bpf.RetConstant{Val: 256},
	}
}

// applyToSocket applies BFP program to the socket
func (filter BPF) applyToSocket(socket int) (err error) {
	// thanks to Riyaz Ali
	// https://riyazali.net/posts/berkeley-packet-filter-in-golang/
	var assembled []bpf.RawInstruction
	if assembled, err = bpf.Assemble(filter); err != nil {
		err = fmt.Errorf("can't assemble BPF filter: %w", err)
		return err
	}

	var program = unix.SockFprog{
		Len:    uint16(len(assembled)),
		Filter: (*unix.SockFilter)(unsafe.Pointer(&assembled[0])),
	}
	var b = (*[unix.SizeofSockFprog]byte)(unsafe.Pointer(&program))[:unix.SizeofSockFprog]

	if _, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(socket), uintptr(syscall.SOL_SOCKET), uintptr(syscall.SO_ATTACH_FILTER),
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0); errno != 0 {
		return errno
	}

	return nil
}
