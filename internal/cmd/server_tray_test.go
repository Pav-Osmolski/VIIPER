package cmd

import (
	"context"
	"os"
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
			noTray := serverTrayDisabled(cli.Server.NoTray, false, func() bool { return true })
			if noTray != (mode != "default") {
				t.Fatal("incorrect tray option")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started, stopped := false, false
			stop := startServerTray(ctx, cancel, noTray, func(context.Context, func()) func() {
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

func TestTrayPreferencePrecedenceAndIsolation(t *testing.T) {
	t.Setenv("VIIPER_NO_TRAY", "")
	if err := os.Unsetenv("VIIPER_NO_TRAY"); err != nil {
		t.Fatal(err)
	}
	var cli struct {
		Server Server `cmd:""`
	}
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.Parse([]string{"server"}); err != nil {
		t.Fatal(err)
	}
	if cli.Server.NoTray != nil {
		t.Fatal("Kong supplied an implicit override for the saved preference")
	}
	for _, visible := range []bool{true, false} {
		read := func() bool { return visible }
		if serverTrayDisabled(nil, false, read) == visible {
			t.Fatal("saved Windows preference ignored")
		}
		for _, explicit := range []bool{true, false} {
			if serverTrayDisabled(&explicit, false, read) != explicit {
				t.Fatal("saved preference overrode explicit flag/environment")
			}
		}
	}
	if serverTrayDisabled(nil, true, func() bool { t.Fatal("isolated server read installed preference"); return false }) {
		t.Fatal("explicit-key deployment implicitly hid tray")
	}
}
