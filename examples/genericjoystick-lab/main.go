// genericjoystick-lab is the bounded stdin/stdout feeder used by the Windows
// test launcher. It cannot install drivers, change startup, or detach ports.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/genericjoystick"
	"github.com/Alia5/VIIPER/viiperclient"
)

func decodeFrame(line string, count int) (genericjoystick.InputState, error) {
	var state genericjoystick.InputState
	if len(line) != genericjoystick.ReportSize*2 {
		return state, fmt.Errorf("frame must contain exactly 32 hex characters")
	}
	b, err := hex.DecodeString(line)
	if err != nil {
		return state, err
	}
	if err := state.UnmarshalBinary(b); err != nil {
		return state, err
	}
	return state, state.Validate(count)
}

func emit(value any) error {
	return json.NewEncoder(os.Stdout).Encode(value)
}

func run() error {
	addr := flag.String("api", "", "isolated literal loopback API host:port")
	key := flag.String("key-file", "", "absolute lab-only password file")
	options := flag.String("options-file", "", "absolute creation options JSON file")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("lab API must be a literal loopback address")
	}
	if !filepath.IsAbs(*key) || !filepath.IsAbs(*options) {
		return fmt.Errorf("explicit absolute lab key and options paths are required")
	}
	password, err := os.ReadFile(*key)
	if err != nil {
		return fmt.Errorf("cannot read lab key file")
	}
	if strings.TrimSpace(string(password)) == "" {
		return fmt.Errorf("empty lab key file")
	}
	args, err := os.ReadFile(*options)
	if err != nil {
		return err
	}
	var parsed genericjoystick.CreateOptions
	if err := json.Unmarshal(args, &parsed); err != nil {
		return err
	}
	// Validate with the same constructor before any management operation.
	if _, err := genericjoystick.New(&device.CreateOptions{DeviceSpecific: string(args)}); err != nil {
		return err
	}
	neutral := *genericjoystick.NewInputState()
	if len(parsed.NeutralAxes) == 6 {
		neutral.X, neutral.Y, neutral.Z = parsed.NeutralAxes[0], parsed.NeutralAxes[1], parsed.NeutralAxes[2]
		neutral.Rx, neutral.Ry, neutral.Rz = parsed.NeutralAxes[3], parsed.NeutralAxes[4], parsed.NeutralAxes[5]
	}
	client := viiperclient.NewWithPassword(*addr, strings.TrimSpace(string(password)))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	bus, err := client.BusCreateCtx(ctx, 0)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := client.BusRemoveCtx(ctx, bus.BusID); err != nil {
			fmt.Fprintln(os.Stderr, "remove lab bus:", err)
		}
	}()
	// The launcher must learn the bus before native attachment. A closed
	// stdout must not leave an import without a known ownership identity.
	if err := emit(map[string]any{"kind": "allocated", "bus": bus.BusID}); err != nil {
		return err
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	receipt, err := client.DeviceAddCtx(ctx, bus.BusID, "genericjoystick", &device.CreateOptions{DeviceSpecific: string(args)})
	cancel()
	if err != nil {
		return err
	}
	// Windows configures the newly attached HID device asynchronously. Its
	// SET_CONFIGURATION legitimately retires any earlier producer lease.
	// Let the launcher prove enumeration/neutral before it opens this stream.
	var stream *viiperclient.DeviceStream
	var mu sync.Mutex
	current := neutral
	var writeError error
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(8 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				mu.Lock()
				if stream != nil && writeError == nil {
					writeError = stream.WriteBinary(current)
				}
				mu.Unlock()
			}
		}
	}()
	defer func() {
		close(stop)
		<-done
		mu.Lock()
		defer mu.Unlock()
		if stream != nil {
			_ = stream.Close()
		}
	}()
	if err := emit(map[string]any{"kind": "ready", "bus": bus.BusID, "device": receipt.DevID, "port": receipt.USBIPPort, "vid": receipt.Vid, "pid": receipt.Pid}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "quit" {
			return emit(map[string]any{"kind": "stopped"})
		}
		mu.Lock()
		switch line {
		case "disconnect":
			if stream == nil {
				err = fmt.Errorf("already disconnected")
			} else {
				err = stream.Close()
				stream = nil
				writeError = nil
			}
		case "reconnect":
			if stream != nil {
				err = fmt.Errorf("already connected")
			} else {
				current = neutral
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				stream, err = client.OpenStream(ctx, bus.BusID, receipt.DevID)
				cancel()
				writeError = nil
			}
		default:
			var next genericjoystick.InputState
			next, err = decodeFrame(line, parsed.ButtonCount)
			if err == nil && stream == nil {
				err = fmt.Errorf("no input stream")
			}
			if err == nil && writeError != nil {
				err = writeError
			}
			if err == nil {
				current = next
				err = stream.WriteBinary(current)
			}
		}
		mu.Unlock()
		if err != nil {
			return err
		}
		if err := emit(map[string]any{"kind": "ok", "command": line}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	if err := run(); err != nil {
		_ = emit(map[string]any{"kind": "error", "message": err.Error()})
		os.Exit(1)
	}
}
