package meter

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	contract "github.com/KlinkerD/Ems/contracts/meter"
	"github.com/KlinkerD/Ems/contracts/modbus"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/modbustcp"
)

// fakeServer beantwortet FC 0x03/0x04 aus zwei Registerbänken.
type fakeServer struct {
	holding, input [256]uint16
	requests       atomic.Int32
	conns          atomic.Int32
}

func (s *fakeServer) start(t *testing.T) (host string, port uint16) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.conns.Add(1)
			go s.serve(conn)
		}
	}()
	h, p, _ := net.SplitHostPort(ln.Addr().String())
	n, _ := strconv.Atoi(p)
	return h, uint16(n)
}

func (s *fakeServer) serve(conn net.Conn) {
	defer conn.Close()
	for {
		req := make([]byte, 12)
		if _, err := io.ReadFull(conn, req); err != nil {
			return
		}
		s.requests.Add(1)
		fc := req[7]
		addr := binary.BigEndian.Uint16(req[8:])
		count := binary.BigEndian.Uint16(req[10:])

		bank := &s.holding
		if fc == 0x04 {
			bank = &s.input
		}
		var pdu []byte
		if int(addr)+int(count) > len(bank) {
			pdu = []byte{fc | 0x80, 0x02}
		} else {
			pdu = []byte{fc, byte(count * 2)}
			for _, r := range bank[addr : addr+count] {
				pdu = binary.BigEndian.AppendUint16(pdu, r)
			}
		}
		resp := append([]byte{}, req[0:4]...)
		resp = binary.BigEndian.AppendUint16(resp, uint16(len(pdu)+1))
		resp = append(resp, req[6])
		resp = append(resp, pdu...)
		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

func (s *fakeServer) putFloat32(bank *[256]uint16, addr int, v float32) {
	bits := math.Float32bits(v)
	bank[addr] = uint16(bits >> 16)
	bank[addr+1] = uint16(bits)
}

func (s *fakeServer) putString(bank *[256]uint16, addr int, str string) {
	for i := 0; i < len(str); i += 2 {
		hi, lo := str[i], byte(0)
		if i+1 < len(str) {
			lo = str[i+1]
		}
		bank[addr+i/2] = uint16(hi)<<8 | uint16(lo)
	}
}

func float32Reg(addr uint16) modbus.NumberRegister {
	return modbus.NumberRegister{
		Address: addr, Length: 2, Type: modbus.InputRegister,
		DataType: modbus.Float32,
	}
}

func TestReadValues(t *testing.T) {
	srv := &fakeServer{}
	srv.putFloat32(&srv.input, 0, 230.5)
	srv.putFloat32(&srv.input, 2, 231)
	srv.putFloat32(&srv.input, 4, 229.5)
	srv.putFloat32(&srv.input, 72, 12.5)
	host, port := srv.start(t)

	m := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: host, Port: port, UnitID: 1,
		PollingInterval: modbus.Duration{Duration: 10 * time.Millisecond},
		Profile: modbus.Profile{
			DeviceType: "test",
			MeterValues: map[contract.Type]modbus.NumberRegister{
				contract.U1: float32Reg(0),
				contract.U2: float32Reg(2),
				contract.U3: float32Reg(4),
				contract.ActiveEnergyImported: func() modbus.NumberRegister {
					r := float32Reg(72)
					r.Scale = 1000 // kWh → Wh
					return r
				}(),
			},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan []contract.Value)
	errCh := make(chan error, 1)
	go func() { errCh <- m.ReadValues(ctx, ch) }()

	got := map[contract.Type]float64{}
	for _, v := range <-ch {
		if v.MeterID != contract.MustMeterID("m1") {
			t.Errorf("MeterID %q", v.MeterID)
		}
		got[v.Type] = v.Value
	}
	want := map[contract.Type]float64{
		contract.U1: 230.5, contract.U2: 231, contract.U3: 229.5, contract.ActiveEnergyImported: 12500,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: got %v, want %v", k, got[k], w)
		}
	}

	// U1..U3 liegen zusammen, Energie separat → 2 Anfragen pro Poll.
	if n := srv.requests.Load(); n != 2 {
		t.Errorf("requests nach erstem Poll: %d, want 2", n)
	}

	<-ch // zweiter Poll kommt über den Ticker
	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestReadValuesModbusException(t *testing.T) {
	srv := &fakeServer{}
	host, port := srv.start(t)
	m := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: host, Port: port,
		PollingInterval: modbus.Duration{Duration: time.Second},
		Profile: modbus.Profile{
			DeviceType:  "test",
			MeterValues: map[contract.Type]modbus.NumberRegister{contract.U1: float32Reg(255)},
		},
	})
	err := m.ReadValues(context.Background(), make(chan []contract.Value, 1))
	var ex modbustcp.Exception
	if !errors.As(err, &ex) || ex.Code != 0x02 {
		t.Fatalf("err = %v, want modbustcp.Exception 0x02", err)
	}
}

func TestReadDeviceInfo(t *testing.T) {
	srv := &fakeServer{}
	srv.putString(&srv.holding, 10, "SN12345\x00")
	srv.putString(&srv.holding, 20, "Eastron ")
	host, port := srv.start(t)

	m := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: host, Port: port,
		Profile: modbus.Profile{
			DeviceType: "test",
			DeviceInfo: map[modbus.DeviceInfoField]modbus.StringRegister{
				modbus.SerialNumber: {Address: 10, Length: 4},
				modbus.Manufacturer: {Address: 20, Length: 4},
			},
		},
	})
	ch := make(chan contract.DeviceInfo, 1)
	if err := m.ReadDeviceInfo(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	info := <-ch
	if info.MeterID != contract.MustMeterID("m1") || info.SerialNumber != "SN12345" || info.Manufacturer != "Eastron" {
		t.Errorf("got %+v", info)
	}
}

func TestReadTimeout(t *testing.T) {
	// Server nimmt an, antwortet aber nie.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			defer c.Close()
			io.Copy(io.Discard, c)
		}
	}()
	_, p, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(p)

	m := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: "127.0.0.1", Port: uint16(port),
		ReadTimeout:     modbus.Duration{Duration: 50 * time.Millisecond},
		PollingInterval: modbus.Duration{Duration: time.Second},
		Profile: modbus.Profile{
			DeviceType:  "test",
			MeterValues: map[contract.Type]modbus.NumberRegister{contract.U1: float32Reg(0)},
		},
	})
	start := time.Now()
	err = m.ReadValues(context.Background(), make(chan []contract.Value, 1))
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("err = %v, want timeout", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("timeout dauerte %v", d)
	}
}

// TestReaderSharesConnection prüft den Ablauf aus poller.Run: Geräteinfo und
// Messwerte laufen über dieselbe Verbindung, nach einem Fehler wird neu
// verbunden.
func TestReaderSharesConnection(t *testing.T) {
	srv := &fakeServer{}
	srv.putString(&srv.holding, 10, "SN1\x00")
	srv.putFloat32(&srv.input, 0, 230)
	host, port := srv.start(t)

	r := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: host, Port: port,
		PollingInterval: modbus.Duration{Duration: time.Hour},
		Profile: modbus.Profile{
			DeviceType: "test",
			DeviceInfo: map[modbus.DeviceInfoField]modbus.StringRegister{
				modbus.SerialNumber: {Address: 10, Length: 2},
			},
			MeterValues: map[contract.Type]modbus.NumberRegister{contract.U1: float32Reg(0)},
		},
	})

	// Eine Verbindungsrunde wie in poller.Run, beendet durch ctx.
	round := func() {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		infoCh := make(chan contract.DeviceInfo, 1)
		if err := r.ReadDeviceInfo(ctx, infoCh); err != nil {
			t.Fatalf("ReadDeviceInfo: %v", err)
		}
		if info := <-infoCh; info.SerialNumber != "SN1" {
			t.Errorf("SerialNumber = %q, want SN1", info.SerialNumber)
		}
		valCh := make(chan []contract.Value)
		errCh := make(chan error, 1)
		go func() { errCh <- r.ReadValues(ctx, valCh) }()
		if vals := <-valCh; len(vals) != 1 || vals[0].Value != 230 {
			t.Errorf("values = %+v", vals)
		}
		cancel()
		if err := <-errCh; !errors.Is(err, context.Canceled) {
			t.Errorf("ReadValues err = %v, want context.Canceled", err)
		}
	}

	round()
	if n := srv.conns.Load(); n != 1 {
		t.Fatalf("verbindungen nach erster Runde: %d, want 1", n)
	}

	// Nach dem Ende von ReadValues ist die Verbindung zu, die nächste Runde
	// verbindet neu.
	round()
	if n := srv.conns.Load(); n != 2 {
		t.Fatalf("verbindungen nach zweiter Runde: %d, want 2", n)
	}
}

// TestReadDeviceInfoWithoutRegisters prüft, dass ohne Geräteinfo-Register im
// Profil nicht verbunden wird.
func TestReadDeviceInfoWithoutRegisters(t *testing.T) {
	r := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: "127.0.0.1", Port: 1,
		Profile: modbus.Profile{DeviceType: "test"},
	})
	ch := make(chan contract.DeviceInfo, 1)
	if err := r.ReadDeviceInfo(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	if info := <-ch; info.MeterID != contract.MustMeterID("m1") {
		t.Errorf("got %+v", info)
	}
	if r.conn != nil {
		t.Error("verbindung offen, obwohl keine Geräteinfo-Register")
	}
}

// TestReadDeviceInfoErrorClosesConnection prüft, dass nach einem Lesefehler
// keine halb kaputte Verbindung offen bleibt.
func TestReadDeviceInfoErrorClosesConnection(t *testing.T) {
	srv := &fakeServer{}
	host, port := srv.start(t)
	r := NewReader(&modbus.Config{
		ID: contract.MustMeterID("m1"), Host: host, Port: port,
		Profile: modbus.Profile{
			DeviceType: "test",
			DeviceInfo: map[modbus.DeviceInfoField]modbus.StringRegister{
				// Adresse außerhalb der Registerbank → Modbus-Exception.
				modbus.SerialNumber: {Address: 255, Length: 4},
			},
		},
	})
	if err := r.ReadDeviceInfo(context.Background(), make(chan contract.DeviceInfo, 1)); err == nil {
		t.Fatal("err = nil, want Fehler")
	}
	if r.conn != nil {
		t.Error("verbindung nach Fehler noch offen")
	}
}
