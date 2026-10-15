//go:build windows

package cmd

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func TestInstalledTrayPreferenceDefaultsAndDword(t *testing.T) {
	path := fmt.Sprintf(`Software\DS4Windows-Tests\Tray-%d`, time.Now().UnixNano())
	if !readSavedTrayVisible(path) {
		t.Fatal("missing key must show tray")
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = key.Close(); _ = registry.DeleteKey(registry.CURRENT_USER, path) }()
	if !readSavedTrayVisible(path) {
		t.Fatal("missing value must show tray")
	}
	for _, value := range []uint32{0, 1} {
		if err := key.SetDWordValue("ShowViiperTrayIcon", value); err != nil {
			t.Fatal(err)
		}
		if readSavedTrayVisible(path) != (value != 0) {
			t.Fatal("saved DWORD ignored")
		}
	}
	if err := key.SetStringValue("ShowViiperTrayIcon", "false"); err != nil {
		t.Fatal(err)
	}
	if !readSavedTrayVisible(path) {
		t.Fatal("invalid value must keep tray visible")
	}
}
