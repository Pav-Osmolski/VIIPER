package api

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net"
	"strings"
	"syscall"
	"testing"
)

type traceTestWriter struct {
	n   int
	err error
}

func (w traceTestWriter) Write(p []byte) (int, error) {
	if w.n < 0 {
		return len(p), w.err
	}
	return w.n, w.err
}

func TestReplyTraceReportsCompletionWithoutContents(t *testing.T) {
	for _, failure := range []bool{false, true} {
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		first, second := net.Pipe()
		defer first.Close()
		defer second.Close()
		trace := newConnectionTrace(logger, first)
		trace.authenticated = true
		trace.operation = traceOperation("bus/123/add")
		writer := traceTestWriter{n: -1}
		if failure {
			writer = traceTestWriter{n: 2, err: syscall.Errno(10054)}
		}
		text := "SECRET-RESPONSE\n"
		n, err := trace.write(writer, []byte(text))
		trace.closeBegin()
		trace.closeEnd(nil)
		if failure {
			require.Error(t, err)
			require.Equal(t, 2, n)
			require.Contains(t, logs.String(), "writeFailed=true")
			require.Contains(t, logs.String(), "socketError=10054")
		} else {
			require.NoError(t, err)
			require.Equal(t, len(text), n)
			require.Contains(t, logs.String(), "completedWireBytes=48")
		}
		require.Contains(t, logs.String(), "api reply write begin")
		require.Contains(t, logs.String(), "api reply write end")
		require.Contains(t, logs.String(), "api connection close begin")
		require.Contains(t, logs.String(), "api connection close end")
		require.NotContains(t, logs.String(), "SECRET")
		require.NotContains(t, logs.String(), "bus/123")
	}
}

func TestTraceDoesNotReclassifyCapabilityOrUnknownPath(t *testing.T) {
	for _, path := range []string{"SECRET-CAPABILITY", "bus/x/add", "bus/1/SECRET", "bus/1/a/microphone-interface"} {
		require.Equal(t, "other", traceOperation(path))
	}
	require.Equal(t, "device-add-authorized", traceOperation("bus/1/add-authorized-xboxone"))
}

func TestShortWriteIsRecordedWithoutRetry(t *testing.T) {
	var logs bytes.Buffer
	trace := &connectionTrace{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	n, err := trace.write(traceTestWriter{n: 1}, []byte("test"))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.True(t, trace.replyFailed)
	require.Contains(t, logs.String(), "writeFailed=true")
	require.Equal(t, 1, strings.Count(logs.String(), "api reply write end"))
}

var _ io.Writer = tracedReplyWriter{}
