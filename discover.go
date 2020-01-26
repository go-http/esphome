package esphome

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// Discovery defaults.
const (
	DefaultMDNSService = "_esphomelib._tcp"
	DefaultMDNSDomain  = "local"
	DefaultMDNSTimeout = 5 * time.Second

	mDNSIP4               = "224.0.0.251"
	mDNSIP6               = "ff02::fb"
	mDNSPort              = 5353
	forceUnicastResponses = false
)

var (
	mDNSAddr4 = &net.UDPAddr{IP: net.ParseIP(mDNSIP4), Port: mDNSPort}
	mDNSAddr6 = &net.UDPAddr{IP: net.ParseIP(mDNSIP6), Port: mDNSPort}
)

// Device is an ESPHome device (returned by Discover).
type Device struct {
	Name    string
	Host    string
	Port    int
	IP      net.IP
	IP6     net.IP
	Version string

	sent bool
}

// Addr returns the device API address.
func (d *Device) Addr() string {
	if d.IP6 != nil {
		return (&net.TCPAddr{IP: d.IP6, Port: d.Port}).String()
	} else if d.IP != nil {
		return (&net.TCPAddr{IP: d.IP, Port: d.Port}).String()
	}
	return net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
}

func (d *Device) complete() bool {
	return d.Host != "" && (d.IP != nil || d.IP6 != nil)
}

// Discover ESPHome deices on the network.
func Discover(devices chan<- *Device) error {
	return DiscoverService(devices, DefaultMDNSService, DefaultMDNSDomain, DefaultMDNSTimeout)
}

// DiscoverService is used by Discover, can be used to override the default service and domain and to customize timeouts.
func DiscoverService(devices chan<- *Device, service, domain string, timeout time.Duration) error {
	c, err := newmDNSClient()
	if err != nil {
		return err
	}
	defer c.Close()

	return c.query(devices, service, domain, timeout)
}

type mDNSClient struct {
	// Unicast
	uc4, uc6 *net.UDPConn

	// Multicast
	mc4, mc6 *net.UDPConn

	closed   int32
	closedCh chan struct{}
}

func newmDNSClient() (*mDNSClient, error) {
	uc4, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	uc6, _ := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6zero, Port: 0})
	if uc4 == nil && uc6 == nil {
		return nil, errors.New("esphome: failed to bind to any unicast UDP port")
	}

	mc4, _ := net.ListenMulticastUDP("udp4", nil, mDNSAddr4)
	mc6, _ := net.ListenMulticastUDP("udp6", nil, mDNSAddr6)
	if uc4 == nil && uc6 == nil {
		if uc4 != nil {
			_ = uc4.Close()
		}
		if uc6 != nil {
			_ = uc6.Close()
		}
		return nil, errors.New("esphome: failed to bind to any multicast TCP port")
	}

	return &mDNSClient{
		uc4:      uc4,
		uc6:      uc6,
		mc4:      mc4,
		mc6:      mc6,
		closedCh: make(chan struct{}),
	}, nil
}

func (c *mDNSClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		// something else already closed it
		return nil
	}
	close(c.closedCh)
	if c.uc4 != nil {
		_ = c.uc4.Close()
	}
	if c.uc6 != nil {
		_ = c.uc6.Close()
	}
	if c.mc4 != nil {
		_ = c.mc4.Close()
	}
	if c.mc6 != nil {
		_ = c.mc6.Close()
	}

	return nil
}

/*
query performs mDNS service discovery:

  client:  question _esphomelib._tcp.local. IN PTR?
  servers: response _esphomelib._tcp.local. IN PTR  <hostname>._esphomelib._tcp.local
         <hostname>._esphomelib._tcp.local. IN SRV  <hostname>.local. <port> 0 0
         <hostname>._esphomelib._tcp.local. IN TXT  <record>*
                          <hostname>.local. IN A    <ipv4 address>
                          <hostname>.local. IN AAAA <ipv6 address>
*/
func (c *mDNSClient) query(devices chan<- *Device, service, domain string, timeout time.Duration) error {
	// Create the service name
	addr := fmt.Sprintf("%s.%s", strings.Trim(service, "."), strings.Trim(domain, "."))

	// Start listening for response packets
	var (
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		msgs        = make(chan *dns.Msg, 32)
	)
	defer cancel()
	go c.recv(ctx, c.uc4, msgs)
	go c.recv(ctx, c.uc6, msgs)
	go c.recv(ctx, c.mc4, msgs)
	go c.recv(ctx, c.mc6, msgs)

	q := new(dns.Msg)
	q.SetQuestion(addr+".", dns.TypePTR)
	// RFC 6762, section 18.12.  Repurposing of Top Bit of qclass in Question
	// Section
	//
	// In the Question Section of a Multicast DNS query, the top bit of the qclass
	// field is used to indicate that unicast responses are preferred for this
	// particular question.  (See Section 5.4.)
	q.Question[0].Qclass |= 1 << 15
	q.RecursionDesired = false
	if err := c.send(q); err != nil {
		return err
	}

	var partial = make(map[string]*Device)
	for ctx.Err() == nil {
		select {
		case replies := <-msgs:
			var d *Device
			for _, a := range append(replies.Answer, replies.Extra...) {
				switch rr := a.(type) {
				case *dns.PTR:
					d = c.parsePTR(addr, partial, rr)
				case *dns.SRV:
					d = c.parseSRV(addr, partial, rr)
				case *dns.A:
					d = c.parseA(domain, partial, rr)
				case *dns.AAAA:
					d = c.parseAAAA(domain, partial, rr)
				case *dns.TXT:
					d = c.parseTXT(addr, partial, rr)
				}
			}

			if d == nil {
				continue
			}

			if d.complete() {
				//log.Printf("complete device: %#+v", d)
				if !d.sent {
					select {
					case devices <- d:
					default:
					}
					d.sent = true
				}
			}

		case <-ctx.Done():
			return nil
		}
	}

	return ctx.Err()
}

func (c *mDNSClient) parsePTR(addr string, partial map[string]*Device, rr *dns.PTR) *Device {
	// _esphomelib._tcp.local. IN PTR  <hostname>._esphomelib._tcp.local
	index := strings.IndexByte(rr.Ptr, '.')
	if index == -1 {
		return nil
	}

	hostname := rr.Ptr[:index]
	if !strings.EqualFold(strings.Trim(rr.Hdr.Name, "."), addr) {
		return nil
	}

	return ensureDevice(partial, hostname)
}

func (c *mDNSClient) parseSRV(addr string, partial map[string]*Device, rr *dns.SRV) *Device {
	// <hostname>._esphomelib._tcp.local. IN SRV  <hostname>.local. <port> 0 0
	index := strings.IndexByte(rr.Hdr.Name, '.')
	if index == -1 {
		return nil
	}

	hostname, suffix := rr.Hdr.Name[:index], strings.Trim(rr.Hdr.Name[index+1:], ".")
	if !strings.EqualFold(suffix, addr) {
		log.Printf("%q != %q", suffix, addr)
		return nil
	}

	d := ensureDevice(partial, hostname)
	d.Host = rr.Target
	d.Port = int(rr.Port)
	return d
}

func (c *mDNSClient) parseTXT(addr string, partial map[string]*Device, rr *dns.TXT) *Device {
	// <hostname>._esphomelib._tcp.local. IN TXT  <record>*
	index := strings.IndexByte(rr.Hdr.Name, '.')
	if index == -1 {
		return nil
	}

	hostname, suffix := rr.Hdr.Name[:index], strings.Trim(rr.Hdr.Name[index+1:], ".")
	if !strings.EqualFold(suffix, addr) {
		log.Printf("%q != %q", suffix, addr)
		return nil

	}

	d := ensureDevice(partial, hostname)
	for _, t := range rr.Txt {
		if i := strings.IndexByte(t, '='); i > -1 {
			switch t[:i] {
			case "address":
				d.Host = t[i+1:]
			case "version":
				d.Version = t[i+1:]
			}
		}
	}
	return d
}

func (c *mDNSClient) parseA(domain string, partial map[string]*Device, rr *dns.A) *Device {
	// <hostname>.local. IN A    <ipv4 address>
	index := strings.IndexByte(rr.Hdr.Name, '.')
	if index == -1 {
		return nil
	}

	hostname, suffix := rr.Hdr.Name[:index], strings.Trim(rr.Hdr.Name[index+1:], ".")
	if !strings.EqualFold(suffix, domain) {
		log.Printf("%q != %q", suffix, domain)
		return nil
	}

	d := ensureDevice(partial, hostname)
	d.IP = rr.A
	return d
}

func (c *mDNSClient) parseAAAA(domain string, partial map[string]*Device, rr *dns.AAAA) *Device {
	// <hostname>.local. IN AAAA <ipv6 address>
	index := strings.IndexByte(rr.Hdr.Name, '.')
	if index == -1 {
		return nil
	}

	hostname, suffix := rr.Hdr.Name[:index], strings.Trim(rr.Hdr.Name[index+1:], ".")
	if !strings.EqualFold(suffix, domain) {
		log.Printf("%q != %q", suffix, domain)
		return nil
	}

	d := ensureDevice(partial, hostname)
	d.IP6 = rr.AAAA
	return d
}

func ensureDevice(partial map[string]*Device, name string) *Device {
	name = strings.Trim(name, ".")
	if d, ok := partial[name]; ok {
		return d
	}

	d := &Device{Name: name, Port: DefaultPort}
	partial[name] = d
	return d
}

func ensureDeviceAlias(partial map[string]*Device, src, dst string) {
	partial[dst] = ensureDevice(partial, src)
}

func (c *mDNSClient) recv(ctx context.Context, l *net.UDPConn, msgCh chan *dns.Msg) {
	if l == nil {
		return
	}

	buf := make([]byte, 65536)
	for ctx.Err() == nil {
		n, err := l.Read(buf)
		if err != nil {
			continue
		}
		msg := new(dns.Msg)
		if err := msg.Unpack(buf[:n]); err != nil {
			continue
		}
		select {
		case msgCh <- msg:
		case <-ctx.Done():
			return
		}
	}
}

func (c *mDNSClient) send(q *dns.Msg) error {
	buf, err := q.Pack()
	if err != nil {
		return err
	}

	if c.uc4 != nil {
		c.uc4.WriteToUDP(buf, mDNSAddr4)
	}
	if c.uc6 != nil {
		c.uc6.WriteToUDP(buf, mDNSAddr6)
	}

	return nil
}
