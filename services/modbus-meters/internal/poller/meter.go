// Package poller startet und stoppt Meter zur Laufzeit und verbindet sie nach
// Fehlern neu. Es kennt nur das Interface Meter, nicht das Protokoll dahinter.
package poller

import (
	"context"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// Meter enthält nur, was Manager und Run tatsächlich aufrufen.
type Meter interface {
	MeterID() string
	Validate() error
	ReadDeviceInfo(ctx context.Context, ch chan<- contract.DeviceInfo) error
	ReadValues(ctx context.Context, ch chan<- []contract.Value) error
}
