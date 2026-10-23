// This diagnostic helper never starts a server, changes a driver or creates
// a virtual device. It independently tests authenticated loopback ping replies.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"github.com/Alia5/VIIPER/internal/server/api/auth"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type attempt struct {
	Phase       string `json:"phase"`
	Success     bool   `json:"success"`
	LocalPort   int    `json:"localPort"`
	SocketError int    `json:"socketError"`
	ElapsedMs   int64  `json:"elapsedMs"`
}
type result struct {
	Implementation string    `json:"implementation"`
	Attempted      int       `json:"attempted"`
	Successful     int       `json:"successful"`
	Attempts       []attempt `json:"attempts"`
	SetupFailed    bool      `json:"setupFailed"`
}

func main() {
	keyFile := flag.String("key-file", "", "Existing API password file (read only)")
	count := flag.Int("count", 20, "Number of authenticated ping requests (1-20)")
	port := flag.Int("port", 3242, "Loopback API port")
	flag.Parse()
	summary := result{Implementation: "go-independent"}
	defer func() { _ = json.NewEncoder(os.Stdout).Encode(summary) }()
	// Bound file reads and never emit the path, file contents or error messages.
	if *count < 1 || *count > 20 || *port < 1 || *port > 65535 || !filepathAbsolute(*keyFile) {
		summary.SetupFailed = true
		return
	}
	file, err := os.Open(*keyFile)
	if err != nil {
		summary.SetupFailed = true
		return
	}
	bytes, err := io.ReadAll(io.LimitReader(file, 4097))
	_ = file.Close()
	if err != nil || len(bytes) > 4096 || strings.TrimSpace(string(bytes)) == "" {
		summary.SetupFailed = true
		clear(bytes)
		return
	}
	key, err := auth.DeriveKey(strings.TrimSpace(string(bytes)))
	clear(bytes)
	if err != nil {
		summary.SetupFailed = true
		return
	}
	defer clear(key)
	for i := 0; i < *count; i++ {
		next := ping(net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)), key)
		summary.Attempts = append(summary.Attempts, next)
		summary.Attempted++
		if next.Success {
			summary.Successful++
		}
		if i+1 < *count {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func ping(address string, key []byte) (r attempt) {
	started := time.Now()
	r.Phase = "Connect"
	defer func() { r.ElapsedMs = time.Since(started).Milliseconds() }()
	fail := func(err error) {
		var number syscall.Errno
		if errors.As(err, &number) {
			r.SocketError = int(number)
		}
	}
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		fail(err)
		return
	}
	defer conn.Close()
	r.LocalPort = conn.LocalAddr().(*net.TCPAddr).Port
	if err = conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		fail(err)
		return
	}
	r.Phase = "Authenticate"
	client, server, err := auth.HandleAuthHandshake(bufio.NewReader(conn), conn, key, true)
	if err != nil {
		fail(err)
		return
	}
	session := auth.DeriveSessionKey(key, server, client)
	defer clear(session)
	defer clear(client)
	defer clear(server)
	secure, err := auth.WrapConn(conn, session, auth.Client)
	if err != nil {
		fail(err)
		return
	}
	r.Phase = "WritePing"
	if _, err = io.WriteString(secure, "ping\x00"); err != nil {
		fail(err)
		return
	}
	r.Phase = "ReadReply"
	reply, err := bufio.NewReader(io.LimitReader(secure, 4096)).ReadString('\n')
	if err != nil {
		fail(err)
		return
	}
	if !strings.Contains(strings.ToUpper(reply), "VIIPER") {
		return
	}
	r.Success = true
	return
}

// Never load configuration or touch default key paths.
func filepathAbsolute(path string) bool { return filepath.IsAbs(path) }
