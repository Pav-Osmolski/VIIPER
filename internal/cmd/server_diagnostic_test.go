package cmd

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/internal/log"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/auth"
	"github.com/stretchr/testify/require"
)

func diagnosticConfig(t *testing.T) Server {
	t.Helper()
	key := filepath.Join(t.TempDir(), "lab.key")
	require.NoError(t, os.WriteFile(key, []byte("isolated-test-key"), 0600))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := ln.Addr().String()
	ln.Close()
	return Server{DiagnosticPingOnly: true, KeyFile: &key, ConnectionTimeout: time.Second,
		APIServerConfig: api.ServerConfig{Addr: address, RequireLocalHostAuth: true, ManagementCloseMode: "graceful"}}
}

func TestDiagnosticPingRejectsUnsafeConfigurations(t *testing.T) {
	for name, change := range map[string]func(*Server){
		"default port": func(s *Server) { s.APIServerConfig.Addr = "127.0.0.1:3242" },
		"USB port":     func(s *Server) { s.APIServerConfig.Addr = "127.0.0.1:3240" },
		"wildcard":     func(s *Server) { s.APIServerConfig.Addr = "0.0.0.0:40404" },
		"hostname":     func(s *Server) { s.APIServerConfig.Addr = "localhost:40404" },
		"ephemeral":    func(s *Server) { s.APIServerConfig.Addr = "127.0.0.1:0" },
		"no auth":      func(s *Server) { s.APIServerConfig.RequireLocalHostAuth = false },
		"auto attach":  func(s *Server) { s.APIServerConfig.AutoAttachLocalClient = true },
		"default key":  func(s *Server) { s.KeyFile = nil },
		"authority":    func(s *Server) { s.USBServerConfig.RetainedImportAuthorityID = 1 },
		"missing key":  func(s *Server) { require.NoError(t, os.Remove(*s.KeyFile)) },
		"empty key":    func(s *Server) { require.NoError(t, os.WriteFile(*s.KeyFile, nil, 0600)) },
	} {
		t.Run(name, func(t *testing.T) {
			s := diagnosticConfig(t)
			change(&s)
			_, err := s.diagnosticPingPassword()
			require.Error(t, err)
			if name == "missing key" {
				_, err := os.Stat(*s.KeyFile)
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}

func TestDiagnosticServerUsesAuthenticatedPingWithoutDeviceRoutes(t *testing.T) {
	s := diagnosticConfig(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, closeServer, err := s.newDiagnosticPingServer(logger, log.NewRaw(nil))
	require.NoError(t, err)
	defer closeServer()
	for _, path := range []string{"bus/create", "bus/1/add", "bus/1/remove", "bus/list", "bus/1/1", "bus/1/1/stream-authorized-xboxone"} {
		h, _ := server.Router().Match(path)
		require.Nil(t, h, path)
		hs, _ := server.Router().MatchStream(path)
		require.Nil(t, hs, path)
	}
	require.NoError(t, server.Start())
	c, err := net.DialTimeout("tcp", server.Addr(), time.Second)
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.SetDeadline(time.Now().Add(2*time.Second)))
	key, err := auth.DeriveKey("isolated-test-key")
	require.NoError(t, err)
	client, nonce, err := auth.HandleAuthHandshake(bufio.NewReader(c), c, key, true)
	require.NoError(t, err)
	secure, err := auth.WrapConn(c, auth.DeriveSessionKey(key, nonce, client), auth.Client)
	require.NoError(t, err)
	_, err = io.WriteString(secure, "ping\x00")
	require.NoError(t, err)
	reply, err := bufio.NewReader(secure).ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, reply, "VIIPER")
}
