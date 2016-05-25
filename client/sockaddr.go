package main

import (
	"encoding/binary"
	"net"
	"syscall"
)

type Sockaddr struct {
	IP   net.IP
	Port uint16
}

func uint16toh(i uint16) uint16 {
	return (i << 8) | (i >> 8)
}

func parseRawSockaddr(buf []byte) *Sockaddr {
	if len(buf) < 8 {
		// too short for IPv4
		return nil
	}

	addr := &Sockaddr{
		Port: (uint16(buf[2]) << 8) | uint16(buf[3]),
	}

	switch buf[1] {
	case syscall.AF_INET:
		// IPv4
		addr.IP = net.IP(buf[4:8])
	case syscall.AF_INET6:
		// IPv6
		if len(buf) < 20 {
			return nil
		}
		addr.IP = net.IP(buf[4:20])
	default:
		return nil
	}

	return addr
}

func parseSockaddr(buf []byte) *Sockaddr {
	if len(buf) != 18 {
		return nil
	}

	return &Sockaddr{
		IP:   net.IP(buf[0:16]),
		Port: (uint16(buf[16]) << 8) | uint16(buf[17]),
	}
}

func (addr *Sockaddr) Write(out []byte) {
	copy(out, addr.IP.To16())
	binary.BigEndian.PutUint16(out[16:], uint16(addr.Port))
}

func (addr *Sockaddr) RawFixed() (raw [18]byte) {
	addr.Write(raw[:])
	return
}

func (addr *Sockaddr) Raw() []byte {
	raw := addr.RawFixed()
	return raw[:]
}

func (addr *Sockaddr) Equal(other *Sockaddr) bool {
	return addr.Port == other.Port && addr.IP.Equal(other.IP)
}

// Returns the address family
func (addr *Sockaddr) Family() int {
	if len(addr.IP) == net.IPv4len || (len(addr.IP) > 11 &&
		isZeros(addr.IP[0:10]) && addr.IP[10] == 0xff && addr.IP[11] == 0xff) {
		return syscall.AF_INET
	} else {
		return syscall.AF_INET6
	}
}

func isZeros(p net.IP) bool {
	for _, b := range p {
		if b != 0 {
			return false
		}
	}
	return true
}
