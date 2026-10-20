// Package poller startet und stoppt Meter zur Laufzeit und verbindet sie nach
// Fehlern neu. Es kennt nur das Interface Meter, nicht das Protokoll dahinter.
package poller

import (
	"context"

	"modbus/internal/measurement"
)

// Meter enthält nur, was Manager und Run tatsächlich aufrufen.
type Meter interface {
	MeterID() string
	Validate() error
	ReadDeviceInfo(ctx context.Context, ch chan<- measurement.DeviceInfo) error
	ReadValues(ctx context.Context, ch chan<- []measurement.Value) error
}
