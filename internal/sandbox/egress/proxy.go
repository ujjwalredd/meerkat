// Package egress implements a per-run HTTP/HTTPS forward proxy that
// enforces the policy's domain allow/block lists. HTTPS is handled via
// the HTTP CONNECT method with TLS SNI inspection (no TLS interception).
// Plain HTTP is handled by parsing the Host header.
//
// Intended deployment:
//  1. Start Proxy on 127.0.0.1:<random> bound to the supervised process.
//  2. Set HTTP_PROXY / HTTPS_PROXY in the child's env to point at it.
//  3. On Linux, pair with `bwrap --unshare-net` + slirp4netns so the
//     proxy is the only egress route (closes env-var bypass).
package egress

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ujjwalredd/meerkat/internal/audit"
	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/networkpolicy"
)

type Proxy struct {
	ln     net.Listener
	cfg    *config.NetworkCfg
	audit  *audit.Logger
	mu     sync.Mutex
	closed bool
}

// Start binds 127.0.0.1:<random> and serves forever in a goroutine.
func Start(cfg *config.NetworkCfg, lg *audit.Logger) (*Proxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("egress: listen: %w", err)
	}
	p := &Proxy{ln: ln, cfg: cfg, audit: lg}
	go p.serve()
	return p, nil
}

func (p *Proxy) Addr() string { return p.ln.Addr().String() }

func (p *Proxy) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.ln.Close()
}

func (p *Proxy) serve() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.handle(c)
	}
}

func (p *Proxy) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(60 * time.Second))
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method == http.MethodConnect {
		p.handleConnect(c, br, req)
		return
	}
	p.handlePlain(c, br, req)
}

func (p *Proxy) handleConnect(c net.Conn, br *bufio.Reader, req *http.Request) {
	host, port, err := net.SplitHostPort(req.RequestURI)
	if err != nil {
		host = req.RequestURI
		port = "443"
	}
	_ = port
	// Send 200 OK so the client begins the TLS handshake we can sniff.
	if _, err := c.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		return
	}
	// Peek the ClientHello to extract SNI for verification.
	peek, err := br.Peek(2048)
	sni := ""
	if err == nil {
		sni = parseSNI(peek)
	}
	target := host
	if sni != "" {
		target = sni
		// Defeat domain fronting: SNI must match CONNECT host (allow IDN equiv).
		if !strings.EqualFold(stripPort(host), sni) {
			p.deny(c, host, "SNI/host mismatch: connect="+host+" sni="+sni)
			return
		}
	}
	v := networkpolicy.Evaluate([]string{target}, p.cfg)
	if !v.Allowed {
		p.deny(c, target, v.Reason)
		return
	}
	upstream, err := net.DialTimeout("tcp", net.JoinHostPort(target, port), 10*time.Second)
	if err != nil {
		p.deny(c, target, "dial upstream: "+err.Error())
		return
	}
	defer upstream.Close()
	p.log("network_egress", target, true, "")
	// Splice buffered + remaining traffic.
	go func() {
		_, _ = io.Copy(upstream, br)
	}()
	_, _ = io.Copy(c, upstream)
}

func (p *Proxy) handlePlain(c net.Conn, br *bufio.Reader, req *http.Request) {
	host := stripPort(req.Host)
	v := networkpolicy.Evaluate([]string{host}, p.cfg)
	if !v.Allowed {
		p.deny(c, host, v.Reason)
		return
	}
	// Pass through: dial upstream, rewrite request, copy both ways.
	upstream, err := net.DialTimeout("tcp", net.JoinHostPort(host, "80"), 10*time.Second)
	if err != nil {
		p.deny(c, host, "dial: "+err.Error())
		return
	}
	defer upstream.Close()
	p.log("network_egress", host, true, "")
	if err := req.Write(upstream); err != nil {
		return
	}
	go func() { _, _ = io.Copy(upstream, br) }()
	_, _ = io.Copy(c, upstream)
}

func (p *Proxy) deny(c net.Conn, host, reason string) {
	p.log("network_egress", host, false, reason)
	_, _ = c.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"))
}

func (p *Proxy) log(typ, host string, allowed bool, reason string) {
	if p.audit == nil {
		return
	}
	p.audit.Log(audit.Event{
		Type: typ,
		Extra: map[string]any{
			"host":    host,
			"allowed": allowed,
			"reason":  reason,
		},
	})
}

func stripPort(h string) string {
	if i := strings.IndexByte(h, ':'); i >= 0 {
		return h[:i]
	}
	return h
}

// parseSNI extracts the server name from a TLS ClientHello byte slice.
// Best-effort; returns "" on parse failure.
func parseSNI(b []byte) string {
	// TLS record: type(1) ver(2) len(2) handshake...
	if len(b) < 5 || b[0] != 0x16 {
		return ""
	}
	rec := int(binary.BigEndian.Uint16(b[3:5]))
	if rec+5 > len(b) {
		// best-effort with what we have
	}
	hs := b[5:]
	// Handshake: type(1) len(3) version(2) random(32) sid_len(1)...
	if len(hs) < 38 || hs[0] != 0x01 {
		return ""
	}
	pos := 38
	if pos >= len(hs) {
		return ""
	}
	sidLen := int(hs[pos])
	pos++
	pos += sidLen
	if pos+2 > len(hs) {
		return ""
	}
	csLen := int(binary.BigEndian.Uint16(hs[pos : pos+2]))
	pos += 2 + csLen
	if pos+1 > len(hs) {
		return ""
	}
	compLen := int(hs[pos])
	pos += 1 + compLen
	if pos+2 > len(hs) {
		return ""
	}
	extLen := int(binary.BigEndian.Uint16(hs[pos : pos+2]))
	pos += 2
	if pos+extLen > len(hs) {
		extLen = len(hs) - pos
	}
	end := pos + extLen
	for pos+4 <= end {
		typ := binary.BigEndian.Uint16(hs[pos : pos+2])
		l := int(binary.BigEndian.Uint16(hs[pos+2 : pos+4]))
		pos += 4
		if pos+l > end {
			return ""
		}
		if typ == 0x00 { // server_name
			body := hs[pos : pos+l]
			// list_len(2) entry_type(1) name_len(2) name...
			if len(body) < 5 {
				return ""
			}
			nameLen := int(binary.BigEndian.Uint16(body[3:5]))
			if 5+nameLen > len(body) {
				return ""
			}
			return string(body[5 : 5+nameLen])
		}
		pos += l
	}
	return ""
}

// Compile-time check: import tls so the std lib resolves on Windows builds
// where the package is used by clients of egress.
var _ = tls.VersionTLS12
