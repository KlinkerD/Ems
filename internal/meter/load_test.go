package meter

import (
	"os"
	"path/filepath"
	"testing"

	"modbus/internal/profile"
)

func TestLoadDirUMG96RM(t *testing.T) {
	ms, err := LoadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := ms["umg96rm-1"]
	if !ok {
		t.Fatalf("umg96rm-1 fehlt, geladen: %v", ms)
	}
	if m.Profile.DeviceType != "Janitza UMG 96 RM" || len(m.Profile.MeterValues) != 17 {
		t.Errorf("profil unerwartet: %s mit %d Werten", m.Profile.DeviceType, len(m.Profile.MeterValues))
	}
	if sn := m.Profile.DeviceInfo[profile.SerialNumber]; sn.Address != 754 || sn.DataType != profile.Uint32 {
		t.Errorf("serialNumber unerwartet: %+v", sn)
	}
	if fw := m.Profile.DeviceInfo[profile.Firmware]; fw.Address != 750 || fw.DataType != profile.Int16 {
		t.Errorf("firmware unerwartet: %+v", fw)
	}
}

func TestLoadDirDuplicateID(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile("testdata/umg96rm-1.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.json", "b.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LoadDir(dir); err == nil {
		t.Error("doppelte id sollte fehlschlagen")
	}
}
