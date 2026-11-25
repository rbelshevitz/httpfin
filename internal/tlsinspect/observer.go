package tlsinspect

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	helloTTL        = time.Minute
	maxPendingHello = 4096
)

type Hello struct {
	ServerName, OfferedALPN, Signature string
	SeenAt                             time.Time
}
type Observer struct {
	mu       sync.Mutex
	byRemote map[string]Hello
}

func NewObserver() *Observer { return &Observer{byRemote: make(map[string]Hello)} }
func (o *Observer) Observe(info *tls.ClientHelloInfo) {
	if info == nil || info.Conn == nil {
		return
	}
	signature := signature(info)
	now := time.Now()
	o.mu.Lock()
	for key, hello := range o.byRemote {
		if now.Sub(hello.SeenAt) > helloTTL {
			delete(o.byRemote, key)
		}
	}
	if len(o.byRemote) >= maxPendingHello {
		for key := range o.byRemote {
			delete(o.byRemote, key)
			break
		}
	}
	o.byRemote[info.Conn.RemoteAddr().String()] = Hello{ServerName: info.ServerName, OfferedALPN: strings.Join(info.SupportedProtos, ","), Signature: signature, SeenAt: now}
	o.mu.Unlock()
}
func (o *Observer) Take(remote string) Hello {
	o.mu.Lock()
	defer o.mu.Unlock()
	h := o.byRemote[remote]
	delete(o.byRemote, remote)
	return h
}
func Config(certFile, keyFile string, observer *Observer) (*tls.Config, error) {
	if observer == nil {
		observer = NewObserver()
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	if _, err := x509.ParseCertificate(cert.Certificate[0]); err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}, GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) { observer.Observe(info); return nil, nil }}, nil
}
func signature(info *tls.ClientHelloInfo) string {
	h := sha256.New()
	for _, v := range info.CipherSuites {
		_, _ = fmt.Fprintf(h, "c%d,", v)
	}
	for _, v := range info.SupportedVersions {
		_, _ = fmt.Fprintf(h, "v%d,", v)
	}
	for _, v := range info.SupportedCurves {
		_, _ = fmt.Fprintf(h, "g%d,", v)
	}
	for _, v := range info.SignatureSchemes {
		_, _ = fmt.Fprintf(h, "s%d,", v)
	}
	_, _ = fmt.Fprintf(h, "alpn=%s", strings.Join(info.SupportedProtos, ","))
	return hex.EncodeToString(h.Sum(nil))[:32]
}
func RemoteKey(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}
func EnvTLSConfigured() (string, string, bool) {
	cert, key := os.Getenv("httpfin_tls_cert"), os.Getenv("httpfin_tls_key")
	return cert, key, cert != "" && key != ""
}
func PortLabel(addr string) string { return strconv.Quote(addr) }
