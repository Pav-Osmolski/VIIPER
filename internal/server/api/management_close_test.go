package api

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	apierror "github.com/Alia5/VIIPER/internal/server/api/error"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/stretchr/testify/require"
)

func managementTCPPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	client, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	server, err := ln.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	return client.(*net.TCPConn), server.(*net.TCPConn)
}

func TestGracefulManagementCloseDeliversReplyAndFin(t *testing.T) {
	client, server := managementTCPPair(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = server.Write([]byte("complete reply\n"))
		gracefulManagementClose(server, slog.Default(), time.Second)
		_ = server.Close()
	}()
	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)))
	reply, err := io.ReadAll(client)
	require.NoError(t, err)
	require.Equal(t, "complete reply\n", string(reply))
	require.NoError(t, client.Close())
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not finish after peer EOF")
	}
}

func TestGracefulManagementCloseBoundsSilentPeer(t *testing.T) {
	_, server := managementTCPPair(t)
	var logs bytes.Buffer
	start := time.Now()
	gracefulManagementClose(server, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})), 40*time.Millisecond)
	require.Less(t, time.Since(start), time.Second)
	require.Contains(t, logs.String(), "timedOut=true")
}

func TestGracefulManagementCloseBoundsPeerInput(t *testing.T) {
	client, server := managementTCPPair(t)
	_, err := client.Write(bytes.Repeat([]byte{1}, 8192))
	require.NoError(t, err)
	var logs bytes.Buffer
	gracefulManagementClose(server, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})), time.Second)
	require.Contains(t, logs.String(), "bytes=4097")
	require.Contains(t, logs.String(), "limitReached=true")
}

func TestAPICloseInterruptsManagementReplyDrain(t *testing.T) {
	server := lifecycleServer(t)
	server.Router().Register("reply", func(_ *Request, response *Response, _ *slog.Logger) error {
		response.JSON = "reply complete"
		return nil
	})
	require.NoError(t, server.Start())
	client, err := net.Dial("tcp", server.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	_, err = client.Write([]byte("reply\x00"))
	require.NoError(t, err)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
	reply, err := bufio.NewReader(client).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "reply complete\n", reply)
	server.Close()
	require.Eventually(t, func() bool {
		server.lifecycleMu.Lock()
		defer server.lifecycleMu.Unlock()
		return len(server.connections) == 0
	}, time.Second, time.Millisecond)
}

func TestAuthenticatedManagementRepliesAndErrorsRetireTheirConnections(t *testing.T) {
	server := lifecycleServer(t)
	server.config.RequireLocalHostAuth = true
	server.config.Password = "synthetic-management-fixture"
	server.Router().Register("reply", func(_ *Request, response *Response, _ *slog.Logger) error {
		response.JSON = "{\"value\":\"complete\"}"
		return nil
	})
	server.Router().Register("failure", func(_ *Request, _ *Response, _ *slog.Logger) error {
		return apierror.ErrBadRequest("synthetic rejection")
	})
	require.NoError(t, server.Start())
	client := viiperclient.NewTransportWithPassword(server.Addr(), server.config.Password)
	for i := 0; i < 200; i++ {
		reply, err := client.Do("reply", nil, nil)
		require.NoError(t, err, "authenticated request %d", i)
		require.Equal(t, "{\"value\":\"complete\"}", reply)
	}
	errorReply, err := client.Do("failure", nil, nil)
	require.NoError(t, err)
	require.Contains(t, errorReply, "synthetic rejection")
	require.Eventually(t, func() bool {
		server.lifecycleMu.Lock()
		defer server.lifecycleMu.Unlock()
		return len(server.connections) == 0
	}, time.Second, time.Millisecond)
}
