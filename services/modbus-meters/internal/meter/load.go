package meter

import (
	"fmt"
	"os"
	"path/filepath"

	contract "github.com/KlinkerD/Ems/contracts/meter"
	"github.com/KlinkerD/Ems/contracts/modbus"
)

// Load liest eine Meter-Config aus einer Datei.
func Load(path string) (*modbus.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := modbus.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// LoadDir liest alle *.json-Dateien eines Verzeichnisses, eine pro Meter.
func LoadDir(dir string) (map[contract.MeterID]*modbus.Config, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	configs := make(map[contract.MeterID]*modbus.Config, len(paths))
	for _, p := range paths {
		c, err := Load(p)
		if err != nil {
			return nil, err
		}
		if _, dup := configs[c.ID]; dup {
			return nil, fmt.Errorf("%s: meter-id %s doppelt", p, c.ID)
		}
		configs[c.ID] = c
	}
	return configs, nil
}
