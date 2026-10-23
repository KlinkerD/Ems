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

	"github.com/KlinkerD/MeterService/services/modbus-meters/internal/measurement"
	"github.com/KlinkerD/MeterService/services/modbus-meters/internal/modbustcp"
	"github.com/KlinkerD/MeterService/services/modbus-meters/internal/profile"
)

// fakeServer beantwortet FC 0x03/0x04 aus zwei Registerbänken.
type fakeServer struct {
	holding, input [256]uint16
	requests       atomic.Int32
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

func float32Reg(addr uint16) profile.NumberRegister {
	return profile.NumberRegister{
		Register: profile.Register{Address: addr, Length: 2, Type: modbustcp.InputRegister},
		DataType: profile.Float32,
	}
}

func TestReadValues(t *testing.T) {
	srv := &fakeServer{}
	srv.putFloat32(&srv.input, 0, 230.5)
	srv.putFloat32(&srv.input, 2, 231)
	srv.putFloat32(&srv.input, 4, 229.5)
	srv.putFloat32(&srv.input, 72, 12.5)
	host, port := srv.start(t)

	m := ModbusMeter{
		ID: "m1", Host: host, Port: port, UnitID: 1,
		PollingInterval: Duration{10 * time.Millisecond},
		Profile: profile.Profile{
			DeviceType: "test",
			MeterValues: map[measurement.Type]profile.NumberRegister{
				measurement.U1: float32Reg(0),
				measurement.U2: float32Reg(2),
				measurement.U3: float32Reg(4),
				measurement.ActiveEnergyImported: func() profile.NumberRegister {
					r := float32Reg(72)
					r.Scale = 1000 // kWh → Wh
					return r
				}(),
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan []measurement.Value)
	errCh := make(chan error, 1)
	go func() { errCh <- m.ReadValues(ctx, ch) }()

	got := map[measurement.Type]float64{}
	for _, v := range <-ch {
		if v.MeterID != "m1" {
			t.Errorf("MeterID %q", v.MeterID)
		}
		got[v.Type] = v.Value
	}
	want := map[measurement.Type]float64{
		measurement.U1: 230.5, measurement.U2: 231, measurement.U3: 229.5, measurement.ActiveEnergyImported: 12500,
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
	m := ModbusMeter{
		ID: "m1", Host: host, Port: port,
		PollingInterval: Duration{time.Second},
		Profile: profile.Profile{
			DeviceType:  "test",
			MeterValues: map[measurement.Type]profile.NumberRegister{measurement.U1: float32Reg(255)},
		},
	}
	err := m.ReadValues(context.Background(), make(chan []measurement.Value, 1))
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

	m := ModbusMeter{
		ID: "m1", Host: host, Port: port,
		Profile: profile.Profile{
			DeviceType: "test",
			DeviceInfo: map[profile.DeviceInfoField]profile.StringRegister{
				profile.SerialNumber: {Register: profile.Register{Address: 10, Length: 4}},
				profile.Manufacturer: {Register: profile.Register{Address: 20, Length: 4}},
			},
		},
	}
	ch := make(chan measurement.DeviceInfo, 1)
	if err := m.ReadDeviceInfo(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	info := <-ch
	if info.MeterID != "m1" || info.SerialNumber != "SN12345" || info.Manufacturer != "Eastron" {
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

	m := ModbusMeter{
		ID: "m1", Host: "127.0.0.1", Port: uint16(port),
		ReadTimeout:     Duration{50 * time.Millisecond},
		PollingInterval: Duration{time.Second},
		Profile: profile.Profile{
			DeviceType:  "test",
			MeterValues: map[measurement.Type]profile.NumberRegister{measurement.U1: float32Reg(0)},
		},
	}
	start := time.Now()
	err = m.ReadValues(context.Background(), make(chan []measurement.Value, 1))
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("err = %v, want timeout", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("timeout dauerte %v", d)
	}
}
