package api

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestGracefulManagementCloseDeliversReplyAndFin(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		_, _ = c.Write([]byte("complete reply\n"))
		gracefulManagementClose(c, slog.Default(), time.Second)
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	reply, err := io.ReadAll(c)
	require.NoError(t, err)
	require.Equal(t, "complete reply\n", string(reply))
	c.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not finish after peer EOF")
	}
}

func TestGracefulManagementCloseHasBoundedSilentPeer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer c.Close()
	s, err := ln.Accept()
	require.NoError(t, err)
	defer s.Close()
	var logs bytes.Buffer
	start := time.Now()
	gracefulManagementClose(s, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})), 40*time.Millisecond)
	require.Less(t, time.Since(start), time.Second)
	require.Contains(t, logs.String(), "timedOut=true")
}

func TestGracefulManagementCloseInterruptedByOwnedServerShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer c.Close()
	s, err := ln.Accept()
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { gracefulManagementClose(s, slog.Default(), time.Second); close(done) }()
	s.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("socket close did not interrupt drain")
	}
}
