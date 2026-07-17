package main

import "encoding/json"

// Message types - must match server/tcp/protocol.go
const (
	MsgHandshake    = 0x01
	MsgHandshakeAck = 0x02

	MsgLogin    = 0x10
	MsgLoginOK  = 0x11
	MsgLoginErr = 0x12

	MsgQueueSubmit   = 0x20
	MsgQueueSubmitOK = 0x21

	MsgQueuePoll   = 0x30
	MsgQueueStatus = 0x31

	MsgQueueFetchNext = 0x40
	MsgQueueItem      = 0x41
	MsgQueueEmpty     = 0x42

	MsgQueueMarkDone = 0x50
	MsgQueueDoneOK   = 0x51
	MsgQueueDoneErr  = 0x52

	MsgSyncSet    = 0x60
	MsgSyncSetOK  = 0x61
	MsgSyncPoll   = 0x62
	MsgSyncStatus = 0x63

	MsgLicenseCheck  = 0xC0
	MsgLicenseStatus = 0xC1

	MsgHeartbeat    = 0xD0
	MsgHeartbeatAck = 0xD1

	MsgLogUpload   = 0x70
	MsgLogUploadOK = 0x71

	MsgPing = 0xE0
	MsgPong = 0xE1

	MsgDisconnect = 0xFF
)

type Message struct {
	Type    uint8           `json:"t"`
	Payload json.RawMessage `json:"p,omitempty"`
}

type HandshakePayload struct {
	PubKey []byte `json:"pubkey"`
}

type LoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginOKPayload struct {
	HasLicense bool   `json:"has_license"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

type LoginErrPayload struct {
	Reason string `json:"reason"`
}

type QueueSubmitPayload struct {
	YamlData string `json:"yaml_data"`
}

type QueueSubmitOKPayload struct {
	QueueID string `json:"queue_id"`
}

type QueuePollPayload struct {
	QueueID string `json:"queue_id"`
}

type QueueStatusPayload struct {
	QueueID string `json:"queue_id"`
	Status  string `json:"status"`
}

type QueueItemPayload struct {
	QueueID  string `json:"queue_id"`
	YamlData string `json:"yaml_data"`
}

type QueueMarkDonePayload struct {
	QueueID string `json:"queue_id"`
}

type ErrorPayload struct {
	Reason string `json:"reason"`
}

type SyncSetPayload struct {
	Signal string `json:"signal"`
}

type SyncStatusPayload struct {
	Signal string `json:"signal"`
}

type LicenseStatusPayload struct {
	Valid     bool   `json:"valid"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type HeartbeatPayload struct {
	Challenge []byte `json:"c"`
}

type HeartbeatAckPayload struct {
	Response []byte `json:"r"`
}

type LogUploadPayload struct {
	Lines []string `json:"lines"`
}
