// A synthetic feeder for the experimental joystick. The default is dry-run:
// no sockets or devices are opened unless an explicit loopback API is supplied.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/genericjoystick"
	"github.com/Alia5/VIIPER/viiperclient"
)

type step struct {
	name  string
	state genericjoystick.InputState
}

func sequence() []step {
	n := *genericjoystick.NewInputState()
	steps := []step{{"neutral", n}}
	for _, name := range []string{"X", "Y", "Z", "Rx", "Ry", "Rz"} {
		for _, value := range []int16{-32767, 32767} {
			s := n
			switch name {
			case "X":
				s.X = value
			case "Y":
				s.Y = value
			case "Z":
				s.Z = value
			case "Rx":
				s.Rx = value
			case "Ry":
				s.Ry = value
			case "Rz":
				s.Rz = value
			}
			steps = append(steps, step{fmt.Sprintf("%s=%d", name, value), s}, step{"neutral", n})
		}
	}
	for i := 0; i < 12; i++ {
		s := n
		s.Buttons = 1 << i
		steps = append(steps, step{fmt.Sprintf("button %d", i+1), s}, step{"neutral", n})
	}
	for i := uint8(0); i < 8; i++ {
		s := n
		s.Hat = i
		steps = append(steps, step{fmt.Sprintf("hat %d", i), s}, step{"neutral", n})
	}
	return steps
}

func run() error {
	addr := flag.String("api", "", "explicit isolated loopback VIIPER API host:port (empty = dry-run)")
	duration := flag.Duration("hold", 2*time.Second, "hold each diagnostic step (100ms..30s)")
	flag.Parse()
	if *duration < 100*time.Millisecond || *duration > 30*time.Second {
		return fmt.Errorf("hold must be from 100ms through 30s")
	}
	steps := sequence()
	if *addr == "" {
		for _, s := range steps {
			wire, err := s.state.MarshalBinary()
			if err != nil {
				return err
			}
			fmt.Printf("%s\t%s\n", s.name, hex.EncodeToString(wire))
		}
		return nil
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("use a literal loopback IP for an isolated experimental server")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	client := viiperclient.New(*addr)
	// Allocate our own bus; never borrow an existing DS4Windows bus/address.
	bus, err := client.BusCreateCtx(ctx, 0)
	if err != nil {
		return err
	}
	defer func() {
		if _, err := client.BusRemove(bus.BusID); err != nil {
			fmt.Fprintf(os.Stderr, "remove experimental bus %d: %v\n", bus.BusID, err)
		}
	}()
	args, _ := json.Marshal(genericjoystick.CreateOptions{Version: 1, Experimental: true, ButtonCount: 12})
	stream, receipt, err := client.AddDeviceAndConnect(ctx, bus.BusID, "genericjoystick", &device.CreateOptions{DeviceSpecific: string(args)})
	if err != nil {
		return err
	}
	defer func() {
		if err := stream.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close experimental stream: %v\n", err)
		}
	}()
	fmt.Printf("Experimental device %d/%s. No driver installation or usbip attach is performed by this feeder.\n", bus.BusID, receipt.DevID)
	fmt.Println("Use only an isolated lab server. Do not connect to a DS4Windows-managed backend.")
	for _, s := range steps {
		fmt.Println(s.name)
		deadline := time.Now().Add(*duration)
		for time.Now().Before(deadline) {
			if err := stream.WriteBinary(s.state); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(8 * time.Millisecond):
			}
		}
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
