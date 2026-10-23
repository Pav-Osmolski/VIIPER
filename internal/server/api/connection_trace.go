package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var connectionTraceSequence atomic.Uint64

// Only command lifecycle metadata is recorded. Never retain payloads, keys,
// arbitrary paths or error messages. Streaming I/O is not wrapped or traced.
type connectionTrace struct {
	logger                      *slog.Logger
	started                     time.Time
	operation                   string
	authenticated               bool
	replyExpected, replyWritten int
	replyFailed                 bool
}

func newConnectionTrace(logger *slog.Logger, conn net.Conn) *connectionTrace {
	t := &connectionTrace{logger: logger.With("connectionID", connectionTraceSequence.Add(1),
		"remote", conn.RemoteAddr().String()), started: time.Now(), operation: "unclassified"}
	enabled, seconds, err := socketLinger(conn)
	t.logger.Debug("api connection begin", "lingerEnabled", enabled, "lingerSeconds", seconds, "lingerKnown", err == nil)
	return t
}

func traceOperation(path string) string {
	switch path {
	case "ping":
		return "ping"
	case "bus/create":
		return "bus-create"
	case "bus/remove":
		return "bus-remove"
	}
	parts := strings.Split(path, "/")
	if len(parts) == 3 && parts[0] == "bus" {
		if _, err := strconv.Atoi(parts[1]); err == nil {
			switch parts[2] {
			case "add":
				return "device-add"
			case "remove":
				return "device-remove"
			case "add-authorized-xboxone":
				return "device-add-authorized"
			}
		}
	}
	return "other"
}

func traceError(err error) (bool, int) {
	var number syscall.Errno
	if errors.As(err, &number) {
		return err != nil, int(number)
	}
	return err != nil, 0
}

func (t *connectionTrace) write(w io.Writer, p []byte) (int, error) {
	started := time.Now()
	t.replyExpected += len(p)
	t.logger.Debug("api reply write begin", "operation", t.operation, "authenticated", t.authenticated, "plaintextExpected", len(p))
	n, err := w.Write(p)
	t.replyWritten += n
	failed, code := traceError(err)
	t.replyFailed = t.replyFailed || failed || n != len(p)
	level := slog.LevelDebug
	if t.replyFailed {
		level = slog.LevelWarn
	}
	// For this AEAD, a successful nonempty record write includes 32 wire bytes
	// beyond the plaintext. This is socket acceptance, not peer-delivery proof.
	wireCompleted := n
	if err == nil && n == len(p) && n > 0 && t.authenticated {
		wireCompleted += 32
	}
	if err != nil {
		wireCompleted = 0
	}
	t.logger.Log(context.Background(), level, "api reply write end", "operation", t.operation,
		"authenticated", t.authenticated, "plaintextExpected", len(p), "plaintextWritten", n,
		"completedWireBytes", wireCompleted, "writeFailed", t.replyFailed, "socketError", code, "elapsedUs", time.Since(started).Microseconds())
	return n, err
}

type tracedReplyWriter struct {
	io.Writer
	trace *connectionTrace
}

func (w tracedReplyWriter) Write(p []byte) (int, error) { return w.trace.write(w.Writer, p) }

func (t *connectionTrace) closeBegin() {
	t.logger.Debug("api connection close begin", "operation", t.operation, "authenticated", t.authenticated,
		"plaintextExpected", t.replyExpected, "plaintextWritten", t.replyWritten, "replyWriteFailed", t.replyFailed, "elapsedUs", time.Since(t.started).Microseconds())
}
func (t *connectionTrace) closeEnd(err error) {
	failed, code := traceError(err)
	t.logger.Debug("api connection close end", "operation", t.operation, "closeFailed", failed, "socketError", code, "elapsedUs", time.Since(t.started).Microseconds())
}
