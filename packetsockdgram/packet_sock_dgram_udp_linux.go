// create:2021/01/09
// update:2021/02/14
//
package packetsockdgram

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	// "golang.org/x/net/internal/socket"
)

// ===================
// helper

//  package err definition
var (
	ErrNotDestPort    = errors.New("not match dest port")
	ErrNotIpv4        = errors.New("ip ver not 4")
	ErrNotIpv6        = errors.New("ip ver not 6")
	ErrInvalidConn    = errors.New("invalid connection")
	ErrMissingAddress = errors.New("missing address")
	ErrNilHeader      = errors.New("nil header")
	ErrHeaderTooShort = errors.New("header too short")
	ErrPayloadLen     = errors.New("payload size not correct")
)

func htons(host uint16) uint16 {
	return (host&0xff)<<8 | (host >> 8)
}

// ===================
// header
const (
	UDPHeaderLen = 8
)

type IpHeader struct {
	Ipv4Header *ipv4.Header
	Ipv6Header *ipv6.Header
}
type UdpHeader struct {
	SourcePort      int
	DestinationPort int
	Length          int
	Checksum        int
}

func (h *UdpHeader) String() string {
	if h == nil {
		return "<nil>"
	}
	return fmt.Sprintf("srcport=%d dstport=%d len=%d Checksum=%#x", h.SourcePort, h.DestinationPort, h.Length, h.Checksum)
}

// RFC 768
func ParseUDPHeader(b []byte) (*UdpHeader, error) {
	if b == nil {
		return nil, ErrNilHeader
	}
	if len(b) < UDPHeaderLen {
		return nil, ErrHeaderTooShort
	}
	h := new(UdpHeader)

	h.SourcePort = int(binary.BigEndian.Uint16(b[0:2]))
	h.DestinationPort = int(binary.BigEndian.Uint16(b[2:4]))
	h.Length = int(binary.BigEndian.Uint16(b[4:6]))
	h.Checksum = int(binary.BigEndian.Uint16(b[6:8]))
	return h, nil
}

// ===================
// packet
type handler struct {
	fd      int
	sa      syscall.Sockaddr
	rcvport int
	isIpv4  bool
}

func (c *handler) ok() bool { return c != nil }

func (hdl *handler) readFromIf(b []byte) (n int, h *IpHeader, uh *UdpHeader, p []byte, err error) {
	if !hdl.ok() {
		return -1, nil, nil, nil, ErrInvalidConn
	}

	n, _, err = syscall.Recvfrom(hdl.fd, b, syscall.MSG_TRUNC)

	if err != nil {
		return -1, nil, nil, nil, err
	}
	if b[0] != 0x45 {
		return hdl.readFromIpv6(b)
	}
	return hdl.readFromIpv4(b)
}

//
func (hdl *handler) readFrom(b []byte) (n int, h *IpHeader, uh *UdpHeader, p []byte, err error) {

	if !hdl.ok() {
		return -1, nil, nil, nil, ErrInvalidConn
	}

	n, _, err = syscall.Recvfrom(hdl.fd, b, syscall.MSG_TRUNC)

	if err != nil {
		return -1, nil, nil, nil, err
	}

	if hdl.isIpv4 {
		n, h, uh, p, err = hdl.readFromIpv4(b)
	} else {
		n, h, uh, p, err = hdl.readFromIpv6(b)
	}

	return n, h, uh, p, err
}

func (hdl *handler) readFromIpv4(b []byte) (n int, iph *IpHeader, uh *UdpHeader, p []byte, err error) {

	if b[0] != 0x45 { // ip version check
		return n, nil, nil, nil, ErrNotIpv4
	}

	// ipheader
	ipv4h, err := ipv4.ParseHeader(b)
	if err != nil {
		return n, nil, nil, nil, err
	}

	iph = &IpHeader{Ipv4Header: ipv4h}
	endudp4h := ipv4.HeaderLen + UDPHeaderLen

	// udp header
	uh, err = ParseUDPHeader(b[ipv4.HeaderLen:endudp4h])

	if err != nil {
		return n, iph, nil, nil, err
	}

	pb := b[endudp4h:]

	// port
	if hdl.rcvport != uh.DestinationPort {
		return n, iph, uh, pb, ErrNotDestPort
	}

	return n, iph, uh, pb, nil
}
func (hdl *handler) readFromIpv6(b []byte) (n int, iph *IpHeader, uh *UdpHeader, p []byte, err error) {

	if b[0] != 0x60 { // ip version check
		return n, nil, nil, nil, ErrNotIpv6
	}

	// ipheader
	ipv6h, err := ipv6.ParseHeader(b)
	if err != nil {
		return n, nil, nil, nil, err
	}

	iph = &IpHeader{Ipv6Header: ipv6h}

	startudp6h := ipv6.HeaderLen
	endudp6h := startudp6h + UDPHeaderLen

	// udp header
	uh, err = ParseUDPHeader(b[startudp6h:])
	if err != nil {
		return n, iph, nil, nil, err
	}

	pb := b[endudp6h:]
	// port
	if hdl.rcvport != uh.DestinationPort {
		return n, iph, uh, pb, ErrNotDestPort
	}

	return n, iph, uh, pb, nil
}

// ===================
// endpoint
type Conn struct {
	// genericOpt
	fd     int
	sa     syscall.Sockaddr
	isIpv4 bool
	handler
}

func (c *Conn) Close() error {
	if !c.handler.ok() {
		return ErrInvalidConn
	}
	return syscall.Close(c.fd)
}

//
func NewConn(sa syscall.Sockaddr, ifname string) (*Conn, error) {

	isIpv4 := func(sa syscall.Sockaddr) bool {
		switch sa.(type) {
		case *syscall.SockaddrInet4: //
			return true
		case *syscall.SockaddrInet6: //
			return false
		default:
		}
		return false
	}(sa)

	return newConn(sa, isIpv4, ifname)
}

//
func NewConnIf(port int, ifname string) (*Conn, error) {
	return newConnIf(port, ifname)
}

func newConnIf(port int, ifname string) (*Conn, error) {
	intf, err := net.InterfaceByName(ifname)

	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, syscall.ETH_P_ALL)
	if err != nil {
		return nil, err
	}

	sll := syscall.RawSockaddrLinklayer{
		Family:   syscall.AF_PACKET,
		Protocol: htons(uint16(syscall.ETH_P_ALL)),
		Ifindex:  int32(intf.Index),
	}

	_, _, err = syscall.Syscall(syscall.SYS_BIND, uintptr(fd),
		uintptr(unsafe.Pointer(&sll)), unsafe.Sizeof(sll))
	if errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	cnn := &Conn{
		fd:      fd,
		sa:      nil,
		handler: handler{fd, nil, port, false},
	}
	return cnn, nil
}

func newConn(sa syscall.Sockaddr, isIpv4 bool, ifname string) (*Conn, error) {

	ethP := func(b bool) int {
		if b {
			return syscall.ETH_P_IP
		} else {
			return syscall.ETH_P_IPV6
		}
	}(isIpv4)

	intf, err := net.InterfaceByName(ifname)

	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, ethP)
	if err != nil {
		return nil, err
	}

	sll := syscall.RawSockaddrLinklayer{
		Family:   syscall.AF_PACKET,
		Protocol: htons(uint16(ethP)),
		Ifindex:  int32(intf.Index),
	}

	_, _, err = syscall.Syscall(syscall.SYS_BIND, uintptr(fd),
		uintptr(unsafe.Pointer(&sll)), unsafe.Sizeof(sll))
	if errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// port := func(sa syscall.Sockaddr) int {

	// 	if sa.(type) == syscall.SockaddrInet4 {
	// 		return sa.(*syscall.SockaddrInet4).Port
	// 	}
	// 	return sa.(*syscall.SockaddrInet6).Port
	// }(sa)

	cnn := &Conn{
		fd:      fd,
		sa:      sa,
		isIpv4:  isIpv4,
		handler: handler{fd, sa, 55501, isIpv4},
	}
	return cnn, nil
}

func (c *Conn) Readfrom(b []byte) (n int, h *IpHeader, uh *UdpHeader, p []byte, err error) {

	if c.sa == nil {
		return c.handler.readFromIf(b)
	}

	return c.handler.readFrom(b)
}

// ===================
