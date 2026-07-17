package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

type LeagueProxy struct {
	listener net.Listener
	port     int
	ca       *CertAuthority
	quit     chan struct{}
	wg       sync.WaitGroup
}

func NewLeagueProxy() (*LeagueProxy, error) {
	ca, err := NewCertAuthority()
	if err != nil {
		return nil, fmt.Errorf("failed to create CA: %w", err)
	}
	return &LeagueProxy{
		ca:   ca,
		quit: make(chan struct{}),
	}, nil
}

func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

func (p *LeagueProxy) Start() (int, error) {
	port, err := getFreePort()
	if err != nil {
		return 0, err
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return 0, err
	}
	p.listener = ln
	p.port = port

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-p.quit:
					return
				default:
					continue
				}
			}
			go p.handleConnection(conn)
		}
	}()

	return p.port, nil
}

func (p *LeagueProxy) Stop() {
	close(p.quit)
	p.listener.Close()
	p.wg.Wait()
}

func (p *LeagueProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	if req.Method == "CONNECT" {
		p.handleConnect(clientConn, req)
	} else {
		p.handleHTTP(clientConn, req)
	}
}

func (p *LeagueProxy) handleConnect(clientConn net.Conn, req *http.Request) {
	host := req.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	hostname := strings.Split(host, ":")[0]

	if hostname == decodeStr(targetDomainObf) {
		p.handleMITM(clientConn, host)
	} else {
		p.handleTunnel(clientConn, host)
	}
}

func (p *LeagueProxy) handleTunnel(clientConn net.Conn, targetHost string) {
	targetConn, err := net.Dial("tcp", targetHost)
	if err != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(targetConn, clientConn)
	}()
	go func() {
		defer wg.Done()
		io.Copy(clientConn, targetConn)
	}()
	wg.Wait()
}

func (p *LeagueProxy) handleMITM(clientConn net.Conn, targetHost string) {
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	hostname := strings.Split(targetHost, ":")[0]

	tlsCert, err := p.ca.GenerateCert(hostname)
	if err != nil {
		return
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*tlsCert}}
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		return
	}
	defer tlsClientConn.Close()

	clientReader := bufio.NewReader(tlsClientConn)

	for {
		innerReq, err := http.ReadRequest(clientReader)
		if err != nil {
			return
		}

		realConn, err := tls.Dial("tcp", targetHost, &tls.Config{ServerName: hostname})
		if err != nil {
			return
		}

		innerReq.URL.Scheme = "https"
		innerReq.URL.Host = hostname
		innerReq.RequestURI = ""
		innerReq.Header.Del("Accept-Encoding")

		if err := innerReq.Write(realConn); err != nil {
			realConn.Close()
			return
		}

		realReader := bufio.NewReader(realConn)
		resp, err := http.ReadResponse(realReader, innerReq)
		if err != nil {
			realConn.Close()
			return
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		realConn.Close()

		if err != nil {
			return
		}

		modifiedBody, wasModified := ModifyPayload(body, resp.Header)

		if wasModified {
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Transfer-Encoding")
			resp.ContentLength = int64(len(modifiedBody))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
		}

		resp.Body = io.NopCloser(strings.NewReader(string(modifiedBody)))
		resp.Write(tlsClientConn)
	}
}

func (p *LeagueProxy) handleHTTP(clientConn net.Conn, req *http.Request) {
	hostname := req.Host
	if idx := strings.Index(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}

	isConfigRequest := hostname == decodeStr(targetDomainObf) ||
		strings.HasPrefix(req.URL.Path, "/api/v1/config/")

	if isConfigRequest {
		p.handleConfigRequest(clientConn, req)
	} else {
		p.handleHTTPPassthrough(clientConn, req)
	}
}

func (p *LeagueProxy) handleConfigRequest(clientConn net.Conn, req *http.Request) {
	realConn, err := tls.Dial("tcp", decodeStr(targetDomainObf)+":443", &tls.Config{ServerName: decodeStr(targetDomainObf)})
	if err != nil {
		clientConn.Write([]byte("HTTP/1.0 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer realConn.Close()

	req.URL.Scheme = "https"
	req.URL.Host = decodeStr(targetDomainObf)
	req.Host = decodeStr(targetDomainObf)
	req.RequestURI = ""
	req.Header.Del("Accept-Encoding")

	if err := req.Write(realConn); err != nil {
		return
	}

	realReader := bufio.NewReader(realConn)
	resp, err := http.ReadResponse(realReader, req)
	if err != nil {
		return
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return
	}

	modifiedBody, wasModified := ModifyPayload(body, resp.Header)

	if wasModified {
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Transfer-Encoding")
		resp.ContentLength = int64(len(modifiedBody))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
	}

	resp.Body = io.NopCloser(strings.NewReader(string(modifiedBody)))
	resp.Write(clientConn)
}

func (p *LeagueProxy) handleHTTPPassthrough(clientConn net.Conn, req *http.Request) {
	host := req.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}

	targetConn, err := net.Dial("tcp", host)
	if err != nil {
		clientConn.Write([]byte("HTTP/1.0 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	req.Write(targetConn)
	io.Copy(clientConn, targetConn)
}
