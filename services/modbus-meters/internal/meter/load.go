package meter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// Parse dekodiert und prüft eine einzelne Meter-Config, z. B. aus einer
// Datei oder einer MQTT-Nachricht.
func Parse(data []byte) (*ModbusMeter, error) {
	var m ModbusMeter
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Load liest eine Meter-Config aus einer Datei.
func Load(path string) (*ModbusMeter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// LoadDir liest alle *.json-Dateien eines Verzeichnisses, eine pro Meter.
func LoadDir(dir string) (map[contract.MeterID]*ModbusMeter, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	meters := make(map[contract.MeterID]*ModbusMeter, len(paths))
	for _, p := range paths {
		m, err := Load(p)
		if err != nil {
			return nil, err
		}
		if _, dup := meters[m.ID]; dup {
			return nil, fmt.Errorf("%s: meter-id %s doppelt", p, m.ID)
		}
		meters[m.ID] = m
	}
	return meters, nil
}

// Validate prüft die Config, damit Fehler beim Laden auffallen
// und nicht erst beim ersten Poll.
func (m *ModbusMeter) Validate() error {
	if m.ID.IsZero() {
		return errors.New("id fehlt")
	}
	if m.Host == "" {
		return fmt.Errorf("meter %s: host fehlt", m.ID)
	}
	if m.PollingInterval.Duration <= 0 {
		return fmt.Errorf("meter %s: pollingInterval muss > 0 sein", m.ID)
	}
	if err := m.Profile.Validate(); err != nil {
		return fmt.Errorf("meter %s: %w", m.ID, err)
	}
	return nil
}
