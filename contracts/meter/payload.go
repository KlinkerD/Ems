package meter

import "time"

// MeterIDs ist der Payload auf TopicMeterIDs: die IDs aller Meter, deren
// Einzel-Config auf TopicConfig abonniert werden soll.
type MeterIDs struct {
	Version int      `json:"version"`
	Meters  []string `json:"meters"`
}

// Value ist ein einzelner Messwert. TopicValues trägt eine Liste davon.
type Value struct {
	MeterID   string    `json:"meterId"`
	Timestamp time.Time `json:"timestamp"`
	Type      Type      `json:"type"`
	Value     float64   `json:"value"`
}

// DeviceInfo ist der Payload auf TopicInfo. Er wird bei jedem
// Verbindungsaufbau zum Gerät gesendet.
type DeviceInfo struct {
	MeterID      string `json:"meterId"`
	SerialNumber string `json:"serialNumber,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Firmware     string `json:"firmware,omitempty"`
}

// Error ist der Payload auf TopicError, z. B. wenn das Gerät nicht antwortet.
type Error struct {
	MeterID   string    `json:"meterId"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Status ist der Payload auf TopicStatus, z. B. "Started" oder "Invalid Config".
type Status struct {
	MeterID   string    `json:"meterId"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
