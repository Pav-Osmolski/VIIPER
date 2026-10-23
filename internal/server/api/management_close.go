package api

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"time"
)

// Only completed one-shot replies enter this path. Streaming writes and failed
// authentication/replies retain their existing immediate cancellation semantics.
// Keep the accepted connection registered so Server.Close can interrupt a drain.
func gracefulManagementClose(conn net.Conn, logger *slog.Logger, budget time.Duration) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		logger.Debug("api graceful shutdown skipped", "reason", "not-tcp")
		return
	}
	started := time.Now()
	err := tcp.CloseWrite()
	failed, code := traceError(err)
	logger.Debug("api send shutdown", "failed", failed, "socketError", code)
	if err != nil {
		return
	}
	if err = tcp.SetReadDeadline(time.Now().Add(budget)); err != nil {
		return
	}
	// A peer should close on receiving the FIN. Bound both time and input so a
	// broken or hostile peer cannot retain a connection or force unbounded reads.
	n, err := io.CopyN(io.Discard, tcp, 4097)
	var timeout net.Error
	timedOut := errors.As(err, &timeout) && timeout.Timeout()
	failed, code = traceError(err)
	logger.Debug("api graceful drain end", "bytes", n, "peerEOF", errors.Is(err, io.EOF),
		"timedOut", timedOut, "limitReached", n == 4097, "failed", failed && !errors.Is(err, io.EOF),
		"socketError", code, "elapsedUs", time.Since(started).Microseconds())
}
