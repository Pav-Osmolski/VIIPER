package cmd

import (
	"context"
	"testing"

	"github.com/alecthomas/kong"
)

func TestNoTrayCLIAndEnvironment(t *testing.T) {
	for _, mode := range []string{"default", "flag", "environment"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("VIIPER_NO_TRAY", "false")
			args := []string{"server"}
			if mode == "flag" {
				args = append(args, "--no-tray")
			}
			if mode == "environment" {
				t.Setenv("VIIPER_NO_TRAY", "true")
			}
			var cli struct {
				Server Server `cmd:""`
			}
			parser, err := kong.New(&cli)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parser.Parse(args); err != nil {
				t.Fatal(err)
			}
			if cli.Server.NoTray != (mode != "default") {
				t.Fatal("incorrect tray option")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started, stopped := false, false
			stop := startServerTray(ctx, cancel, cli.Server.NoTray, func(context.Context, func()) func() {
				started = true
				return func() { stopped = true }
			})
			stop()
			if started != (mode == "default") || stopped != started {
				t.Fatal("incorrect tray lifecycle")
			}
		})
	}
}
