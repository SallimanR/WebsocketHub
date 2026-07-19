package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/SallimanR/websockethub/test/testutils/network"
	testws "github.com/SallimanR/websockethub/test/testutils/websocket"
	wsPB "github.com/SallimanR/websockethub/websockethub/proto"
)

type mockSession struct {
	userID int64
	roles  []string
}

func (m *mockSession) GetUserID() int64   { return m.userID }
func (m *mockSession) GetRoles() []string { return m.roles }

func mockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.Param("role")
		var userID int64 = 1
		c.Set("user", &mockSession{userID: userID, roles: []string{role}})
		c.Next()
	}
}

func testLogger(t testing.TB) zerolog.Logger {
	t.Helper()
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	return zerolog.New(output).Level(zerolog.InfoLevel).With().Timestamp().Logger()
}

func tempConfig(t testing.TB, port int) *Config {
	t.Helper()
	return &Config{
		Port:  port,
		Roles: []string{"test_role"},
		Channels: []ChannelConfig{
			{Name: "GPS_REALTIME", Roles: []string{"test_role"}},
		},
	}
}

func TestServerStartupAndConfig(t *testing.T) {
	port := network.GetFreePort(t)
	cfg := tempConfig(t, port)

	srv, err := NewServer(WithLogger(testLogger(t)), WithConfig(cfg))
	require.NoError(t, err)

	err = srv.Start()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()
}

func TestServerPublishFlow(t *testing.T) {
	port := network.GetFreePort(t)
	cfg := tempConfig(t, port)

	srv, err := NewServer(WithLogger(testLogger(t)), WithConfig(cfg), WithAuthMiddleware(mockAuthMiddleware()))
	require.NoError(t, err)
	err = srv.Start()
	require.NoError(t, err)
	defer func() { srv.Shutdown(context.Background()) }()
	time.Sleep(100 * time.Millisecond)

	dialer := testws.SetupDialer(t, nil)
	defer testws.ShutdownDialer(dialer)

	wsAddr := fmt.Sprintf("127.0.0.1:%d", port)

	conn := testws.CreateWSConn(t, "test_role", 1, wsAddr, dialer)
	defer conn.Close()

	msg, err := proto.Marshal(&wsPB.Request{
		RequestId: "req-1",
		Channel:   wsPB.Channel_GPS_REALTIME,
		Payload: &wsPB.Request_Publish{
			Publish: &wsPB.PublishMessage{Data: []byte("test data")},
		},
	})
	require.NoError(t, err)

	err = conn.WriteMessage(websocket.BinaryMessage, msg)
	require.NoError(t, err)
}

func TestServerBroadcastFlow(t *testing.T) {
	port := network.GetFreePort(t)
	cfg := tempConfig(t, port)

	srv, err := NewServer(WithLogger(testLogger(t)), WithConfig(cfg), WithAuthMiddleware(mockAuthMiddleware()))
	require.NoError(t, err)
	err = srv.Start()
	require.NoError(t, err)
	defer func() { srv.Shutdown(context.Background()) }()
	time.Sleep(100 * time.Millisecond)

	dialer := testws.SetupDialer(t, nil)
	defer testws.ShutdownDialer(dialer)
	wsAddr := fmt.Sprintf("127.0.0.1:%d", port)

	pubConn := testws.CreateWSConn(t, "test_role", 1, wsAddr, dialer)
	defer pubConn.Close()

	subConn := testws.CreateWSConn(t, "test_role", 2, wsAddr, dialer)
	defer subConn.Close()

	gotBroadcast := make(chan []byte, 1)
	subConn.OnMessage(func(conn *websocket.Conn, msgType websocket.MessageType, data []byte) {
		gotBroadcast <- data
	})

	done := make(chan struct{})
	subMsg, _ := proto.Marshal(&wsPB.Request{
		RequestId: "sub-1",
		Channel:   wsPB.Channel_GPS_REALTIME,
		Payload: &wsPB.Request_Subscribe{
			Subscribe: &wsPB.SubscribeMessage{Indexes: []int64{1}},
		},
	})
	subConn.WriteMessage(websocket.BinaryMessage, subMsg)
	time.Sleep(50 * time.Millisecond)

	pubMsg, _ := proto.Marshal(&wsPB.Request{
		RequestId: "pub-1",
		Channel:   wsPB.Channel_GPS_REALTIME,
		Payload: &wsPB.Request_Publish{
			Publish: &wsPB.PublishMessage{Data: []byte("broadcast data")},
		},
	})
	pubConn.WriteMessage(websocket.BinaryMessage, pubMsg)

	go func() {
		<-gotBroadcast
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func writeTestConfig(t testing.TB, cfg Config) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)
	return path
}
