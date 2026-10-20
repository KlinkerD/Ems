package meters

import (
	"context"
	"modbus/meters/values"
)

// PollableMeter enthält nur, was Manager und Run tatsächlich aufrufen.
type PollableMeter interface {
	MeterID() string
	Validate() error
	ReadDeviceInfo(ctx context.Context, ch chan<- values.DeviceInfo) error
	ReadValues(ctx context.Context, ch chan<- []values.MeterValue) error
}

var _ PollableMeter = (*ModbusMeter)(nil)
