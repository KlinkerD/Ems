package meters

import "time"

// Duration erlaubt "5s" oder "20ms" in JSON statt Nanosekunden.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}
