package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"

	"echo_chat_server/internal/config"
	myredis "echo_chat_server/internal/service/redis"
)

func newTestRedisBackedKafkaServer(t *testing.T) (*KafkaServer, *miniredis.Miniredis, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run returned error: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
		DB:   0,
	})
	restoreRedis := myredis.SetClientForTest(client)

	conf := config.GetConfig()
	prevTTL := conf.KafkaConfig.WSRouteTTLSeconds
	conf.KafkaConfig.WSRouteTTLSeconds = 60

	server := &KafkaServer{
		Clients:    make(map[string]*Client),
		mutex:      &sync.Mutex{},
		instanceID: "chat-test",
	}

	cleanup := func() {
		conf.KafkaConfig.WSRouteTTLSeconds = prevTTL
		restoreRedis()
		_ = client.Close()
		mr.Close()
	}
	return server, mr, cleanup
}

func newTestClientForRoute(uuid string, connectionID string, connectedAt time.Time) *Client {
	return &Client{
		Uuid:         uuid,
		ConnectionID: connectionID,
		connectedAt:  connectedAt,
	}
}

func newTestWebSocketConn(t *testing.T) (*websocket.Conn, func()) {
	t.Helper()

	serverConnCh := make(chan *websocket.Conn, 1)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- conn
		<-done
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		close(done)
		srv.Close()
		t.Fatalf("websocket dial returned error: %v", err)
	}

	serverConn := <-serverConnCh
	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
		close(done)
		srv.Close()
	}
	return clientConn, cleanup
}

func TestRegisterClientRouteWritesStructuredRecordWithTTL(t *testing.T) {
	server, mr, cleanup := newTestRedisBackedKafkaServer(t)
	defer cleanup()

	connectedAt := time.Unix(1710000000, 0)
	client := newTestClientForRoute("U-route-1", "WS-conn-1", connectedAt)

	server.registerClientRoute(client)

	key := kafkaWSRouteKey(client.Uuid)
	raw, err := mr.Get(key)
	if err != nil {
		t.Fatalf("miniredis.Get returned error: %v", err)
	}
	record, err := decodeKafkaWSRouteRecord(raw)
	if err != nil {
		t.Fatalf("decodeKafkaWSRouteRecord returned error: %v", err)
	}
	if record.InstanceID != "chat-test" {
		t.Fatalf("instance_id = %s, want chat-test", record.InstanceID)
	}
	if record.ConnectionID != "WS-conn-1" {
		t.Fatalf("connection_id = %s, want WS-conn-1", record.ConnectionID)
	}
	if record.ConnectedAtUnix != connectedAt.Unix() {
		t.Fatalf("connected_at_unix = %d, want %d", record.ConnectedAtUnix, connectedAt.Unix())
	}
	if ttl := mr.TTL(key); ttl != 60*time.Second {
		t.Fatalf("ttl = %v, want %v", ttl, 60*time.Second)
	}
}

func TestRenewClientRouteRefreshesTTLAndActiveAt(t *testing.T) {
	server, mr, cleanup := newTestRedisBackedKafkaServer(t)
	defer cleanup()

	connectedAt := time.Unix(1710000000, 0)
	client := newTestClientForRoute("U-route-2", "WS-conn-2", connectedAt)

	server.registerClientRoute(client)
	key := kafkaWSRouteKey(client.Uuid)
	mr.FastForward(45 * time.Second)
	if ttl := mr.TTL(key); ttl != 15*time.Second {
		t.Fatalf("ttl before renew = %v, want %v", ttl, 15*time.Second)
	}

	renewAt := connectedAt.Add(50 * time.Second)
	server.renewClientRoute(client, renewAt)

	raw, err := mr.Get(key)
	if err != nil {
		t.Fatalf("miniredis.Get returned error: %v", err)
	}
	record, err := decodeKafkaWSRouteRecord(raw)
	if err != nil {
		t.Fatalf("decodeKafkaWSRouteRecord returned error: %v", err)
	}
	if record.ActiveAtUnix != renewAt.Unix() {
		t.Fatalf("active_at_unix = %d, want %d", record.ActiveAtUnix, renewAt.Unix())
	}
	if record.ConnectedAtUnix != connectedAt.Unix() {
		t.Fatalf("connected_at_unix = %d, want %d", record.ConnectedAtUnix, connectedAt.Unix())
	}
	if ttl := mr.TTL(key); ttl != 60*time.Second {
		t.Fatalf("ttl after renew = %v, want %v", ttl, 60*time.Second)
	}
}

func TestUnregisterOldConnectionDoesNotDeleteNewRoute(t *testing.T) {
	server, mr, cleanup := newTestRedisBackedKafkaServer(t)
	defer cleanup()

	connectedAt := time.Unix(1710000000, 0)
	oldClient := newTestClientForRoute("U-route-3", "WS-old", connectedAt)
	newClient := newTestClientForRoute("U-route-3", "WS-new", connectedAt.Add(5*time.Second))

	server.registerClientRoute(oldClient)
	server.registerClientRoute(newClient)
	server.unregisterClientRoute(oldClient)

	key := kafkaWSRouteKey(newClient.Uuid)
	if !mr.Exists(key) {
		t.Fatalf("expected route to remain after unregistering old connection")
	}
	raw, err := mr.Get(key)
	if err != nil {
		t.Fatalf("miniredis.Get returned error: %v", err)
	}
	record, err := decodeKafkaWSRouteRecord(raw)
	if err != nil {
		t.Fatalf("decodeKafkaWSRouteRecord returned error: %v", err)
	}
	if record.ConnectionID != "WS-new" {
		t.Fatalf("connection_id = %s, want WS-new", record.ConnectionID)
	}
}

func TestRouteTTLExpiresWithoutRenew(t *testing.T) {
	server, mr, cleanup := newTestRedisBackedKafkaServer(t)
	defer cleanup()

	client := newTestClientForRoute("U-route-ttl", "WS-ttl", time.Unix(1710000000, 0))
	server.registerClientRoute(client)

	key := kafkaWSRouteKey(client.Uuid)
	if !mr.Exists(key) {
		t.Fatalf("expected route to exist right after registration")
	}

	mr.FastForward(61 * time.Second)
	if mr.Exists(key) {
		t.Fatalf("expected route to expire after ttl without renew")
	}
}

func TestCloseClientsGracefullyDeletesRedisRoutes(t *testing.T) {
	server, mr, cleanupRedis := newTestRedisBackedKafkaServer(t)
	defer cleanupRedis()

	conn, cleanupConn := newTestWebSocketConn(t)
	defer cleanupConn()

	client := &Client{
		Conn:         conn,
		Uuid:         "U-route-4",
		ConnectionID: "WS-close",
		RoutePath:    "/wss",
		CriticalBack: make(chan *MessageBack, 1),
		SendBack:     make(chan *MessageBack, 1),
		SendTo:       make(chan []byte, 1),
		connectedAt:  time.Unix(1710000000, 0),
		closed:       make(chan struct{}),
	}
	server.Clients[client.Uuid] = client
	server.registerClientRoute(client)

	server.closeClientsGracefully("server shutdown")

	if mr.Exists(kafkaWSRouteKey(client.Uuid)) {
		t.Fatalf("expected route to be deleted during graceful shutdown")
	}
}
