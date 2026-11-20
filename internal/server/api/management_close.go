package api

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"time"
)

// gracefulManagementClose finishes a completed one-shot reply without aborting
// unread peer traffic. Only the raw TCP transport is drained: authentication,
// encrypted framing and device streams retain their existing contracts.
func gracefulManagementClose(conn net.Conn, logger *slog.Logger, budget time.Duration) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	if err := tcp.CloseWrite(); err != nil {
		logger.Debug("management send shutdown failed", "error", err)
		return
	}
	if err := tcp.SetReadDeadline(time.Now().Add(budget)); err != nil {
		return
	}
	// Bound both time and bytes; an uncooperative client must not keep a
	// handler alive. Server.Close can always interrupt this registered socket.
	n, err := io.CopyN(io.Discard, tcp, 4097)
	var netError net.Error
	logger.Debug("management reply drain ended", "bytes", n,
		"peerEOF", errors.Is(err, io.EOF),
		"timedOut", errors.As(err, &netError) && netError.Timeout(),
		"limitReached", n == 4097)
}
