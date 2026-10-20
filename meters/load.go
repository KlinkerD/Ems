package meters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ParseMeter dekodiert und prüft eine einzelne Meter-Config, z. B. aus einer
// Datei oder einer MQTT-Nachricht.
func ParseMeter(data []byte) (*ModbusMeter, error) {
	var m ModbusMeter
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadMeter liest eine Meter-Config aus einer Datei.
func LoadMeter(path string) (*ModbusMeter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := ParseMeter(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// LoadMeters liest alle *.json-Dateien eines Verzeichnisses, eine pro Meter.
func LoadMeters(dir string) (map[string]*ModbusMeter, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	meters := make(map[string]*ModbusMeter, len(paths))
	for _, p := range paths {
		m, err := LoadMeter(p)
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
	if m.ID == "" {
		return fmt.Errorf("id fehlt")
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
