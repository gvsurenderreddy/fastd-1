package fastd

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	DevicePath = "/dev/fastd"

	// Requestable events
	POLLIN  = 0x0001
	POLLPRI = 0x0002
	POLLOUT = 0x0004

	// These events are set if they occur regardless of whether they were requested.
	POLLERR  = 0x0008
	POLLHUP  = 0x0010
	POLLNVAL = 0x0020
)

type PollFd struct {
	fd      int32
	events  int16
	revents int16
}

type KernelServer struct {
	dev       *os.File      // Interface to kernel
	recv      chan *Message // Received messages
	addresses []Sockaddr
}

func NewKernelServer(addresses []Sockaddr) (ServerImpl, error) {
	dev, err := os.OpenFile(DevicePath, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	srv := &KernelServer{
		dev:  dev,
		recv: make(chan *Message, 10),
	}

	for _, address := range addresses {
		// may fail
		srv.ioctl(ioctl_CLOSE, address)

		if err = srv.ioctl(ioctl_BIND, address); err != nil {
			srv.Close()
			return nil, fmt.Errorf("bind() failed:", err)
		}
		srv.addresses = append(srv.addresses, address)
	}

	go srv.readPackets()

	return srv, nil
}

func (srv *KernelServer) ioctl(cmd uintptr, addr Sockaddr) error {
	sa := addr.RawFixed()
	return ioctl(srv.dev.Fd(), cmd, uintptr(unsafe.Pointer(&sa)))
}

func (srv *KernelServer) Read() chan *Message {
	return srv.recv
}

func (srv *KernelServer) Close() {
	if srv.dev != nil {
		srv.dev.Close()
	}
	close(srv.recv)
}

func (srv *KernelServer) Peers() (peers []*Peer) {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Println("failed to load interfaces:", err)
		return
	}
	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "fastd") {
			remote, pubkey, err := GetRemote(iface.Name)
			if err == nil {
				peers = append(peers, &Peer{
					Ifname:    iface.Name,
					Remote:    remote,
					PublicKey: pubkey,
				})
				log.Printf("loaded existing session: iface=%s remote=%s pubkey=%x", iface.Name, remote, pubkey)
			} else {
				log.Println("failed to load session: iface=%s", iface.Name)
			}
		}
	}
	return
}

func (srv *KernelServer) readPackets() error {
	buf := make([]byte, 1500)

	pollFd := PollFd{
		fd:     int32(srv.dev.Fd()),
		events: POLLIN | POLLERR | POLLHUP,
	}

	for {
		num, _, errno := syscall.Syscall(syscall.SYS_POLL, uintptr(unsafe.Pointer(&pollFd)), uintptr(1), 60*1000)

		if errno != 0 {
			return fmt.Errorf("syscall.SYS_POLL failed: %d", errno)
		}

		if num < 0 {
			return fmt.Errorf("poll failed: %d", num)
		}

		if num > 0 && pollFd.revents&POLLHUP > 0 {
			// disconnected
			log.Println("device closed")
			return nil
		}

		n, err := srv.dev.Read(buf)
		if err == io.EOF {
			continue
		}
		if err != nil {
			return err
		}

		data := make([]byte, n)
		copy(data, buf[:n])
		if err = srv.read(data); err != nil {
			log.Println(err)
		}
	}
}

func (srv *KernelServer) read(buf []byte) error {
	// check size
	if len(buf) < 40 {
		return fmt.Errorf("packet too small (%d bytes)", len(buf))
	}

	if msg, err := ParseMessage(buf, true); err != nil {
		return fmt.Errorf("unmarshal failed: %v", err)
	} else {
		srv.recv <- msg
		return nil
	}
}

func (srv *KernelServer) Write(msg *Message) error {
	bytes := msg.Marshal(true)
	_, err := srv.dev.Write(bytes)
	return err
}
