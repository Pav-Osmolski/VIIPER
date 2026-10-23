package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/Alia5/VIIPER/internal/log"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/handler"
	"github.com/Alia5/VIIPER/internal/server/usb"
)

// This explicit diagnostic mode cannot create or attach a device. The normal
// StartServer prerequisite boundary remains unchanged. Read the existing lab
// key without ever generating credentials or consulting default key locations.
func (s *Server) diagnosticPingPassword() (string, error) {
	host, port, err := net.SplitHostPort(s.APIServerConfig.Addr)
	ip := net.ParseIP(host)
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || ip == nil || !ip.IsLoopback() || number < 1 || number > 65535 || number == 3242 || number == 3240 {
		return "", fmt.Errorf("diagnostic ping requires a numeric loopback address and a non-production port")
	}
	if !s.APIServerConfig.RequireLocalHostAuth || s.APIServerConfig.AutoAttachLocalClient || s.KeyFile == nil || s.USBServerConfig.RetainedImportAuthorityID != 0 {
		return "", fmt.Errorf("diagnostic ping requires explicit credentials, authentication, no auto-attach and no retained-import authority")
	}
	path, err := resolveServerKeyFilePath(s.KeyFile)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("diagnostic ping requires an existing key file: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	defer clear(data)
	if err != nil || len(data) > 4096 || strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("diagnostic ping key must be nonempty and at most 4096 bytes")
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Server) newDiagnosticPingServer(logger *slog.Logger, rawLogger log.RawLogger) (*api.Server, func(), error) {
	password, err := s.diagnosticPingPassword()
	if err != nil {
		return nil, nil, err
	}
	cfg := s.APIServerConfig
	cfg.Password = password
	cfg.ConnectionTimeout = s.ConnectionTimeout
	// An unstarted, empty bus owner satisfies the shared API lifecycle. No USB
	// listener, runtime probe, tray or controller operation is started here.
	owner := usb.New(usb.ServerConfig{}, logger, rawLogger)
	server := api.New(owner, cfg.Addr, cfg, logger)
	server.Router().Register("ping", handler.Ping())
	return server, func() { server.Close(); _ = owner.Close() }, nil
}

func (s *Server) startDiagnosticPingServer(ctx context.Context, logger *slog.Logger, rawLogger log.RawLogger) error {
	server, closeServer, err := s.newDiagnosticPingServer(logger, rawLogger)
	if err != nil {
		return err
	}
	defer closeServer()
	if ctx.Err() != nil {
		return nil
	}
	logger.Info("Starting isolated diagnostic ping server; USB/IP and device routes disabled")
	if err := server.Start(); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
