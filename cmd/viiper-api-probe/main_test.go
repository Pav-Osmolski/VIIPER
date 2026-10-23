package main

import (
	"bufio"
	"encoding/json"
	"github.com/Alia5/VIIPER/internal/server/api/auth"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIndependentProbeAcceptsCompleteAuthenticatedReplyAndRejectsTruncation(t *testing.T) {
	key, err := auth.DeriveKey("synthetic-test-password")
	require.NoError(t, err)
	for _, truncate := range []bool{false, true} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			client, server, err := auth.HandleAuthHandshake(bufio.NewReader(conn), conn, key, false)
			if err != nil {
				done <- err
				return
			}
			secure, err := auth.WrapConn(conn, auth.DeriveSessionKey(key, server, client), auth.Server)
			if err != nil {
				done <- err
				return
			}
			_, err = bufio.NewReader(secure).ReadString('\x00')
			if err != nil {
				done <- err
				return
			}
			if truncate {
				_, err = conn.Write([]byte{0, 0, 0, 40, 0, 0, 0})
			} else {
				_, err = io.WriteString(secure, "{\"version\":\"VIIPER-test\"}\n")
			}
			done <- err
		}()
		result := ping(listener.Addr().String(), key)
		require.NoError(t, <-done)
		_ = listener.Close()
		require.Equal(t, !truncate, result.Success)
		require.Equal(t, "ReadReply", result.Phase)
		require.Greater(t, result.LocalPort, 0)
		serialized, err := json.Marshal(result)
		require.NoError(t, err)
		require.False(t, strings.Contains(string(serialized), "password"))
	}
}

func TestProbeRejectsRelativeKeyPath(t *testing.T) {
	require.False(t, filepathAbsolute("viiper.key.txt"))
	require.False(t, filepathAbsolute(""))
}
