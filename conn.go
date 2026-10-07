package main

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	maxMsgSize   = 4 * 1024 * 1024
	dialTimeout  = 10 * time.Second
	readTimeout  = 60 * time.Second
	writeTimeout = 10 * time.Second
)

// SecureConn wraps a TCP connection with AES-256-GCM encryption.
// The AES key is never stored — it is derived on demand from ECDH key material
// and zeroed immediately after each use.
type SecureConn struct {
	conn         net.Conn
	privKey      *ecdh.PrivateKey
	peerPubBytes []byte
	mu           sync.Mutex // protects writes
	rpcMu        sync.Mutex // protects request-response pairs
	stateMu      sync.RWMutex
}

// Connect establishes an encrypted TCP connection to the server.
func Connect() (*SecureConn, error) {
	conn, err := net.DialTimeout("tcp", decodeStr(serverAddrObf), dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	sc := &SecureConn{conn: conn}
	if err := sc.performHandshake(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake failed: %w", err)
	}
	return sc, nil
}

func (sc *SecureConn) performHandshake() error {
	privKey, err := generateECDHKeyPair()
	if err != nil {
		return fmt.Errorf("generate ECDH key: %w", err)
	}

	pubBytes := privKey.PublicKey().Bytes()
	hsPayload, err := json.Marshal(HandshakePayload{PubKey: pubBytes})
	if err != nil {
		return fmt.Errorf("marshal handshake payload: %w", err)
	}

	hsMsg, err := json.Marshal(Message{Type: MsgHandshake, Payload: hsPayload})
	if err != nil {
		return fmt.Errorf("marshal handshake message: %w", err)
	}

	if err := sc.writeRaw(hsMsg); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	if err := sc.conn.SetReadDeadline(time.Now().Add(dialTimeout)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}
	ackData, err := sc.readRaw()
	if err != nil {
		return fmt.Errorf("read handshake ack: %w", err)
	}

	var ackMsg Message
	if err := json.Unmarshal(ackData, &ackMsg); err != nil {
		return fmt.Errorf("decode handshake ack: %w", err)
	}
	if ackMsg.Type != MsgHandshakeAck {
		return fmt.Errorf("expected handshake ack, got %d", ackMsg.Type)
	}

	var ackPayload HandshakePayload
	if err := json.Unmarshal(ackMsg.Payload, &ackPayload); err != nil {
		return fmt.Errorf("decode handshake ack payload: %w", err)
	}

	testKey, err := deriveAESKey(privKey, ackPayload.PubKey)
	if err != nil {
		return fmt.Errorf("derive test key: %w", err)
	}
	zeroKey(testKey)

	sc.stateMu.Lock()
	sc.privKey = privKey
	sc.peerPubBytes = append([]byte(nil), ackPayload.PubKey...)
	sc.stateMu.Unlock()

	return nil
}

// deriveKey derives the AES key on demand from stored ECDH material.
// Caller MUST zero the returned key after use.
func (sc *SecureConn) deriveKey() ([]byte, error) {
	sc.stateMu.RLock()
	defer sc.stateMu.RUnlock()

	if sc.privKey == nil || len(sc.peerPubBytes) == 0 {
		return nil, errors.New("secure connection not initialized")
	}
	return deriveAESKey(sc.privKey, sc.peerPubBytes)
}

// zeroKey wipes a key slice.
func zeroKey(k []byte) {
	for i := range k {
		k[i] = 0
	}
}

func (sc *SecureConn) readRaw() ([]byte, error) {
	var length uint32
	if err := binary.Read(sc.conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length > maxMsgSize {
		return nil, fmt.Errorf("message too large: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(sc.conn, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (sc *SecureConn) writeRaw(data []byte) error {
	if sc == nil || sc.conn == nil {
		return errors.New("connection is nil")
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if err := sc.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	if _, err := sc.conn.Write(header); err != nil {
		return err
	}

	for len(data) > 0 {
		n, err := sc.conn.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}

	return nil
}

// Send sends an encrypted message. AES key is derived, used, and zeroed.
func (sc *SecureConn) Send(msg *Message) error {
	if sc == nil || sc.conn == nil {
		return errors.New("connection is nil")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	key, err := sc.deriveKey()
	if err != nil {
		return err
	}
	encrypted, err := encrypt(key, data)
	zeroKey(key)
	if err != nil {
		return err
	}
	return sc.writeRaw(encrypted)
}

// Receive reads and decrypts a message. AES key is derived, used, and zeroed.
func (sc *SecureConn) Receive() (*Message, error) {
	if sc == nil || sc.conn == nil {
		return nil, errors.New("connection is nil")
	}

	if err := sc.conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, err
	}
	raw, err := sc.readRaw()
	if err != nil {
		return nil, err
	}

	key, err := sc.deriveKey()
	if err != nil {
		return nil, err
	}
	plaintext, err := decrypt(key, raw)
	zeroKey(key)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (sc *SecureConn) sendPayload(msgType uint8, payload interface{}) error {
	p, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return sc.Send(&Message{Type: msgType, Payload: p})
}

// Login authenticates with the server. Returns license info on success.
func (sc *SecureConn) Login(username, password string) (*LoginOKPayload, error) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.sendPayload(MsgLogin, LoginPayload{Username: username, Password: password}); err != nil {
		return nil, err
	}

	resp, err := sc.Receive()
	if err != nil {
		return nil, err
	}

	switch resp.Type {
	case MsgLoginOK:
		var p LoginOKPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode login payload: %w", err)
		}
		return &p, nil
	case MsgLoginErr:
		var p LoginErrPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode login error payload: %w", err)
		}
		return nil, fmt.Errorf("%s", p.Reason)
	default:
		return nil, fmt.Errorf("unexpected response: %d", resp.Type)
	}
}

// QueueSubmit submits YAML data to the server queue.
func (sc *SecureConn) QueueSubmit(yamlData string) (string, error) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.sendPayload(MsgQueueSubmit, QueueSubmitPayload{YamlData: yamlData}); err != nil {
		return "", err
	}

	resp, err := sc.Receive()
	if err != nil {
		return "", err
	}

	if resp.Type == MsgQueueSubmitOK {
		var p QueueSubmitOKPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return "", fmt.Errorf("decode queue submit ok payload: %w", err)
		}
		return p.QueueID, nil
	}

	var e ErrorPayload
	if err := json.Unmarshal(resp.Payload, &e); err != nil {
		return "", fmt.Errorf("decode queue submit error payload: %w", err)
	}
	return "", fmt.Errorf("%s", e.Reason)
}

// QueuePoll checks the status of a queue item.
func (sc *SecureConn) QueuePoll(queueID string) (string, error) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.sendPayload(MsgQueuePoll, QueuePollPayload{QueueID: queueID}); err != nil {
		return "", err
	}

	resp, err := sc.Receive()
	if err != nil {
		return "", err
	}

	if resp.Type == MsgQueueStatus {
		var p QueueStatusPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return "", fmt.Errorf("decode queue status payload: %w", err)
		}
		return p.Status, nil
	}
	return "", fmt.Errorf("unexpected response: %d", resp.Type)
}

// QueueFetchNext fetches the next pending queue item (worker mode).
func (sc *SecureConn) QueueFetchNext() (*QueueItemPayload, error) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.Send(&Message{Type: MsgQueueFetchNext}); err != nil {
		return nil, err
	}

	resp, err := sc.Receive()
	if err != nil {
		return nil, err
	}

	switch resp.Type {
	case MsgQueueItem:
		var p QueueItemPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode queue item payload: %w", err)
		}
		return &p, nil
	case MsgQueueEmpty:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected response: %d", resp.Type)
	}
}

// QueueMarkDone marks a queue item as ready.
func (sc *SecureConn) QueueMarkDone(queueID string) error {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.sendPayload(MsgQueueMarkDone, QueueMarkDonePayload{QueueID: queueID}); err != nil {
		return err
	}

	resp, err := sc.Receive()
	if err != nil {
		return err
	}

	if resp.Type == MsgQueueDoneOK {
		return nil
	}

	var e ErrorPayload
	if err := json.Unmarshal(resp.Payload, &e); err != nil {
		return fmt.Errorf("decode queue mark done error payload: %w", err)
	}
	return fmt.Errorf("%s", e.Reason)
}

// SyncSet sets the sync signal for this user.
func (sc *SecureConn) SyncSet(signal string) error {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.sendPayload(MsgSyncSet, SyncSetPayload{Signal: signal}); err != nil {
		return err
	}

	resp, err := sc.Receive()
	if err != nil {
		return err
	}

	if resp.Type == MsgSyncSetOK {
		return nil
	}
	return fmt.Errorf("sync set failed")
}

// SyncPoll polls the current sync signal for this user.
func (sc *SecureConn) SyncPoll() (string, error) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.Send(&Message{Type: MsgSyncPoll}); err != nil {
		return "", err
	}

	resp, err := sc.Receive()
	if err != nil {
		return "", err
	}

	if resp.Type == MsgSyncStatus {
		var p SyncStatusPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return "", fmt.Errorf("decode sync status payload: %w", err)
		}
		return p.Signal, nil
	}
	return "", fmt.Errorf("unexpected response: %d", resp.Type)
}

// LicenseCheck asks the server if the current license is still valid.
func (sc *SecureConn) LicenseCheck() (*LicenseStatusPayload, error) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.Send(&Message{Type: MsgLicenseCheck}); err != nil {
		return nil, err
	}

	resp, err := sc.Receive()
	if err != nil {
		return nil, err
	}

	if resp.Type == MsgLicenseStatus {
		var p LicenseStatusPayload
		if err := json.Unmarshal(resp.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode license status payload: %w", err)
		}
		return &p, nil
	}

	var e ErrorPayload
	if err := json.Unmarshal(resp.Payload, &e); err != nil {
		return nil, fmt.Errorf("decode license check error payload: %w", err)
	}
	return nil, fmt.Errorf("%s", e.Reason)
}

// Heartbeat sends a cryptographic challenge and verifies the server's HMAC response.
func (sc *SecureConn) Heartbeat() error {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	challenge := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, challenge); err != nil {
		return fmt.Errorf("heartbeat: generate challenge: %w", err)
	}

	if err := sc.sendPayload(MsgHeartbeat, HeartbeatPayload{Challenge: challenge}); err != nil {
		return fmt.Errorf("heartbeat: send: %w", err)
	}

	resp, err := sc.Receive()
	if err != nil {
		return fmt.Errorf("heartbeat: receive: %w", err)
	}

	if resp.Type != MsgHeartbeatAck {
		return fmt.Errorf("heartbeat: unexpected response type: %d", resp.Type)
	}

	var ack HeartbeatAckPayload
	if err := json.Unmarshal(resp.Payload, &ack); err != nil {
		return fmt.Errorf("heartbeat: unmarshal: %w", err)
	}

	key, err := sc.deriveKey()
	if err != nil {
		return fmt.Errorf("heartbeat: derive key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(challenge)
	expected := mac.Sum(nil)
	zeroKey(key)

	if !hmac.Equal(ack.Response, expected) {
		return fmt.Errorf("heartbeat: HMAC verification failed")
	}

	return nil
}

// UploadLogs sends log lines to the server. Best-effort, errors are ignored.
func (sc *SecureConn) UploadLogs(lines []string) {
	sc.rpcMu.Lock()
	defer sc.rpcMu.Unlock()

	if err := sc.sendPayload(MsgLogUpload, LogUploadPayload{Lines: lines}); err != nil {
		return
	}
	_, _ = sc.Receive()
}

// Close sends disconnect and clears key material.
func (sc *SecureConn) Close() {
	if sc == nil || sc.conn == nil {
		return
	}

	_ = sc.Send(&Message{Type: MsgDisconnect})

	sc.stateMu.Lock()
	sc.privKey = nil
	sc.peerPubBytes = nil
	sc.stateMu.Unlock()

	_ = sc.conn.Close()
}

