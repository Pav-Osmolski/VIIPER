package typescript

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alia5/VIIPER/internal/codegen/meta"
	"github.com/Alia5/VIIPER/internal/codegen/scanner"
)

func TestStandaloneIntegerConstantsAreExported(t *testing.T) {
	constants := []scanner.ConstantInfo{
		{Name: "ContractVersion", Type: "int", Value: int64(1)},
		{Name: "ReportSize", Type: "int", Value: int64(16)},
		{Name: "BtnA", Type: "uint16", Value: int64(1)},
		{Name: "BtnB", Type: "uint16", Value: int64(2)},
		{Name: "BtnC", Type: "uint16", Value: int64(4)},
	}
	dir := t.TempDir()
	md := &meta.Metadata{DevicePackages: map[string]*scanner.DeviceConstants{"genericjoystick": {Constants: constants}}}
	if err := generateConstants(slog.New(slog.NewTextHandler(io.Discard, nil)), dir, "genericjoystick", md); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "GenericjoystickConstants.ts"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, expected := range []string{"export const ContractVersion: number = 0x1;", "export const ReportSize: number = 0x10;", "export enum Btn"} {
		if !strings.Contains(s, expected) {
			t.Fatalf("missing %s: %s", expected, s)
		}
	}
	if strings.Contains(s, "export const BtnA") {
		t.Fatal("enum member also exported as a scalar")
	}
}

func TestEmptyConstantsOverwriteStaleFileWithModule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "EmptyConstants.ts")
	if err := os.WriteFile(path, []byte("export const Stale = 1;"), 0600); err != nil {
		t.Fatal(err)
	}
	md := &meta.Metadata{DevicePackages: map[string]*scanner.DeviceConstants{"empty": {}}}
	if err := generateConstants(slog.New(slog.NewTextHandler(io.Discard, nil)), dir, "empty", md); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "export {};") || strings.Contains(string(b), "Stale") {
		t.Fatalf("invalid empty module: %s", b)
	}
}
