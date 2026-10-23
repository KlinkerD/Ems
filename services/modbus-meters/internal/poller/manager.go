package poller

import (
	"context"
	"slices"
	"sync"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// Manager hält die aktiven Meter als map[ID] und startet bzw. stoppt ihre
// Poll-Goroutinen, wenn Configs zur Laufzeit hinzukommen, ersetzt oder
// entfernt werden.
type Manager struct {
	ctx      context.Context
	valuesCh chan<- []contract.Value
	infoCh   chan<- contract.DeviceInfo
	errCh    chan<- contract.Error

	mu      sync.Mutex
	running map[string]*runningMeter
	wg      sync.WaitGroup
}

type runningMeter struct {
	meter  Meter
	cancel context.CancelFunc
	done   chan struct{}
}

// NewManager erzeugt einen leeren Manager. Wird ctx abgebrochen, stoppen alle Meter.
func NewManager(ctx context.Context, valuesCh chan<- []contract.Value, infoCh chan<- contract.DeviceInfo, errCh chan<- contract.Error) *Manager {
	return &Manager{
		ctx:      ctx,
		valuesCh: valuesCh,
		infoCh:   infoCh,
		errCh:    errCh,
		running:  make(map[string]*runningMeter),
	}
}

// Set startet den Meter oder ersetzt eine bestehende Config mit derselben ID.
// Der alte Meter ist gestoppt, bevor der neue startet, damit nie zwei
// Verbindungen zum selben Gerät offen sind. m darf danach nicht mehr verändert werden.
func (mg *Manager) Set(m Meter) error {
	if err := m.Validate(); err != nil {
		return err
	}
	mg.mu.Lock()
	defer mg.mu.Unlock()
	if err := mg.ctx.Err(); err != nil {
		return err
	}

	mg.stopLocked(m.MeterID())

	ctx, cancel := context.WithCancel(mg.ctx)
	r := &runningMeter{meter: m, cancel: cancel, done: make(chan struct{})}
	mg.running[m.MeterID()] = r
	mg.wg.Go(func() {
		defer close(r.done)
		Run(ctx, m, mg.valuesCh, mg.infoCh, mg.errCh)
	})
	return nil
}

// Remove stoppt den Meter und entfernt ihn. Liefert false, wenn die ID unbekannt ist.
func (mg *Manager) Remove(id string) bool {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	return mg.stopLocked(id)
}

func (mg *Manager) stopLocked(id string) bool {
	r, ok := mg.running[id]
	if !ok {
		return false
	}
	r.cancel()
	<-r.done
	delete(mg.running, id)
	return true
}

// Get liefert die aktive Config zu einer ID.
func (mg *Manager) Get(id string) (Meter, bool) {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	r, ok := mg.running[id]
	if !ok {
		return nil, false
	}
	return r.meter, true
}

// Meters liefert eine Kopie der aktiven map[ID]Config.
func (mg *Manager) Meters() map[string]Meter {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	out := make(map[string]Meter, len(mg.running))
	for id, r := range mg.running {
		out[id] = r.meter
	}
	return out
}

// Wait blockiert, bis alle Meter-Goroutinen beendet sind (nach Abbruch von ctx).
func (mg *Manager) Wait() {
	mg.wg.Wait()
}

func (mg *Manager) DeleteMeters(Meters []string) []string {
	var activeMeters = mg.Meters()
	var deletedMeters = make([]string, 0)
	for id := range activeMeters {
		if !slices.Contains(Meters, id) {
			deletedMeters = append(deletedMeters, id)
		}
	}
	for _, id := range deletedMeters {
		mg.Remove(id)
	}
	return deletedMeters
}
