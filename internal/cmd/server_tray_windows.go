//go:build windows

package cmd

import "golang.org/x/sys/windows/registry"

// Per-user UI preference only; never read or change scheduled-task ownership.
func savedTrayVisible() bool {
	return readSavedTrayVisible(`Software\DS4Windows`)
}

func readSavedTrayVisible(path string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer key.Close() //nolint:errcheck
	value, kind, err := key.GetIntegerValue("ShowViiperTrayIcon")
	return err != nil || kind != registry.DWORD || value != 0
}
