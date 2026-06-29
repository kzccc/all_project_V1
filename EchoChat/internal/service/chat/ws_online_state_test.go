package chat

import (
	"sync"
	"testing"
	"time"

	"echo_chat_server/internal/config"
)

func TestDecodeKafkaWSRouteRecordStructured(t *testing.T) {
	raw := `{"instance_id":"chat-8081","connection_id":"WS202606190001","active_at_unix":1710000010,"connected_at_unix":1710000000}`

	record, err := decodeKafkaWSRouteRecord(raw)
	if err != nil {
		t.Fatalf("decodeKafkaWSRouteRecord returned error: %v", err)
	}
	if record.InstanceID != "chat-8081" {
		t.Fatalf("instance_id = %s, want chat-8081", record.InstanceID)
	}
	if record.ConnectionID != "WS202606190001" {
		t.Fatalf("connection_id = %s, want WS202606190001", record.ConnectionID)
	}
	if record.ActiveAtUnix != 1710000010 {
		t.Fatalf("active_at_unix = %d, want 1710000010", record.ActiveAtUnix)
	}
	if record.ConnectedAtUnix != 1710000000 {
		t.Fatalf("connected_at_unix = %d, want 1710000000", record.ConnectedAtUnix)
	}
}

func TestDecodeKafkaWSRouteRecordLegacyString(t *testing.T) {
	record, err := decodeKafkaWSRouteRecord("chat-8081")
	if err != nil {
		t.Fatalf("decodeKafkaWSRouteRecord returned error: %v", err)
	}
	if record.InstanceID != "chat-8081" {
		t.Fatalf("instance_id = %s, want chat-8081", record.InstanceID)
	}
	if record.ConnectionID != "" {
		t.Fatalf("connection_id = %s, want empty", record.ConnectionID)
	}
}

func TestNewKafkaWSRouteRecordUsesClientConnectedAt(t *testing.T) {
	connectedAt := time.Unix(1710000000, 0)
	now := time.Unix(1710000015, 0)
	client := &Client{
		Uuid:         "U1",
		ConnectionID: "WS202606190002",
		connectedAt:  connectedAt,
	}

	record := newKafkaWSRouteRecord(client, "chat-8081", now)
	if record.InstanceID != "chat-8081" {
		t.Fatalf("instance_id = %s, want chat-8081", record.InstanceID)
	}
	if record.ConnectionID != "WS202606190002" {
		t.Fatalf("connection_id = %s, want WS202606190002", record.ConnectionID)
	}
	if record.ActiveAtUnix != now.Unix() {
		t.Fatalf("active_at_unix = %d, want %d", record.ActiveAtUnix, now.Unix())
	}
	if record.ConnectedAtUnix != connectedAt.Unix() {
		t.Fatalf("connected_at_unix = %d, want %d", record.ConnectedAtUnix, connectedAt.Unix())
	}
}

func TestClientRecordCloseReasonKeepsFirstReason(t *testing.T) {
	client := &Client{}

	client.recordCloseReason("heartbeat_timeout")
	client.recordCloseReason("read_failed")

	reasonPtr := client.closeReason.Load()
	if reasonPtr == nil {
		t.Fatalf("expected closeReason to be recorded")
	}
	if *reasonPtr != "heartbeat_timeout" {
		t.Fatalf("closeReason = %s, want heartbeat_timeout", *reasonPtr)
	}
}

func TestClientLastActiveTimeFallsBackAndRefreshes(t *testing.T) {
	connectedAt := time.Unix(1710000000, 0)
	client := &Client{
		connectedAt: connectedAt,
	}

	if got := client.lastActiveTime(); !got.Equal(connectedAt) {
		t.Fatalf("lastActiveTime fallback = %v, want %v", got, connectedAt)
	}

	activeAt := connectedAt.Add(12 * time.Second)
	client.markActive(activeAt)

	if got := client.lastActiveTime(); !got.Equal(activeAt) {
		t.Fatalf("lastActiveTime after markActive = %v, want %v", got, activeAt)
	}
}

func TestHeartbeatTimeoutRemovesClientFromLocalTable(t *testing.T) {
	conf := config.GetConfig()
	prevMode := conf.KafkaConfig.MessageMode
	prevInterval := conf.KafkaConfig.WSHeartbeatIntervalMs
	prevTimeout := conf.KafkaConfig.WSSilenceTimeoutMs
	conf.KafkaConfig.MessageMode = "channel"
	conf.KafkaConfig.WSHeartbeatIntervalMs = 10
	conf.KafkaConfig.WSSilenceTimeoutMs = 25
	defer func() {
		conf.KafkaConfig.MessageMode = prevMode
		conf.KafkaConfig.WSHeartbeatIntervalMs = prevInterval
		conf.KafkaConfig.WSSilenceTimeoutMs = prevTimeout
	}()

	prevChatServer := ChatServer
	testServer := &Server{
		Clients: make(map[string]*Client),
		mutex:   &sync.Mutex{},
	}
	ChatServer = testServer
	defer func() {
		ChatServer = prevChatServer
	}()

	conn, cleanupConn := newTestWebSocketConn(t)
	defer cleanupConn()

	client := &Client{
		Conn:         conn,
		Uuid:         "U-heartbeat-timeout",
		ConnectionID: "WS-heartbeat-timeout",
		RoutePath:    "/wss",
		SendTo:       make(chan []byte, 1),
		SendBack:     make(chan *MessageBack, 1),
		CriticalBack: make(chan *MessageBack, 1),
		connectedAt:  time.Now(),
		closed:       make(chan struct{}),
	}
	testServer.Clients[client.Uuid] = client

	client.startHeartbeat()

	select {
	case <-client.closed:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected heartbeat timeout to close client")
	}

	if got := testServer.GetClient(client.Uuid); got != nil {
		t.Fatalf("expected client to be removed from local table")
	}
	reasonPtr := client.closeReason.Load()
	if reasonPtr == nil {
		t.Fatalf("expected closeReason to be recorded")
	}
	if *reasonPtr != "heartbeat_timeout" {
		t.Fatalf("closeReason = %s, want heartbeat_timeout", *reasonPtr)
	}
}
