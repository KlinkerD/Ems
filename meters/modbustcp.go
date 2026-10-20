package meters

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"time"
)

// maxRegistersPerRead ist das Limit der Modbus-Spezifikation für FC 0x03/0x04.
const maxRegistersPerRead = 125

// tcpClient ist ein minimaler Modbus-TCP-Client, der nur Register liest.
// Er ist nicht für parallele Nutzung gedacht.
type tcpClient struct {
	conn    net.Conn
	unitID  uint8
	timeout time.Duration
	txID    uint16
}

func dialTCP(ctx context.Context, addr string, unitID uint8, timeout time.Duration) (*tcpClient, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return &tcpClient{conn: conn, unitID: unitID, timeout: timeout}, nil
}

func (c *tcpClient) Close() error {
	return c.conn.Close()
}

// ModbusException ist eine Fehlerantwort des Geräts, z. B. Code 2 für ungültige Adresse.
type ModbusException struct {
	Function byte
	Code     byte
}

func (e ModbusException) Error() string {
	return fmt.Sprintf("modbus exception 0x%02x bei Funktion 0x%02x", e.Code, e.Function)
}

// readRegisters liest count Register ab addr und liefert die Rohbytes.
// Nach einem Fehler ist der Zustand der Verbindung unklar, sie sollte
// dann geschlossen werden.
func (c *tcpClient) readRegisters(ctx context.Context, typ RegisterType, addr, count uint16) ([]byte, error) {
	var fc byte
	switch typ {
	case HoldingRegister:
		fc = 0x03
	case InputRegister:
		fc = 0x04
	default:
		return nil, fmt.Errorf("unbekannter registerType %q", typ)
	}
	if count == 0 || count > maxRegistersPerRead {
		return nil, fmt.Errorf("count %d außerhalb 1..%d", count, maxRegistersPerRead)
	}

	deadline := time.Now().Add(c.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	// Abbruch des Kontexts unterbricht laufende Reads/Writes sofort.
	stop := context.AfterFunc(ctx, func() { _ = c.conn.SetDeadline(time.Now()) })
	defer stop()

	c.txID++
	req := make([]byte, 12)
	binary.BigEndian.PutUint16(req[0:], c.txID)
	binary.BigEndian.PutUint16(req[2:], 0) // Protokoll-ID
	binary.BigEndian.PutUint16(req[4:], 6) // Unit-ID + PDU
	req[6] = c.unitID
	req[7] = fc
	binary.BigEndian.PutUint16(req[8:], addr)
	binary.BigEndian.PutUint16(req[10:], count)
	if _, err := c.conn.Write(req); err != nil {
		return nil, c.wrapErr(ctx, err)
	}

	var hdr [7]byte
	if _, err := io.ReadFull(c.conn, hdr[:]); err != nil {
		return nil, c.wrapErr(ctx, err)
	}
	if tx := binary.BigEndian.Uint16(hdr[0:]); tx != c.txID {
		return nil, fmt.Errorf("transaction id %d erwartet, %d erhalten", c.txID, tx)
	}
	length := binary.BigEndian.Uint16(hdr[4:])
	if length < 2 || length > 254 {
		return nil, fmt.Errorf("ungültige MBAP-Länge %d", length)
	}
	pdu := make([]byte, length-1)
	if _, err := io.ReadFull(c.conn, pdu); err != nil {
		return nil, c.wrapErr(ctx, err)
	}

	if pdu[0] == fc|0x80 {
		return nil, ModbusException{Function: fc, Code: pdu[1]}
	}
	if pdu[0] != fc {
		return nil, fmt.Errorf("funktion 0x%02x erwartet, 0x%02x erhalten", fc, pdu[0])
	}
	if int(pdu[1]) != int(count)*2 || len(pdu) != 2+int(count)*2 {
		return nil, fmt.Errorf("%d Bytes erwartet, %d erhalten", count*2, pdu[1])
	}
	return pdu[2:], nil
}

// wrapErr meldet den Kontextfehler statt eines Deadline-Fehlers,
// wenn der Abbruch von außen kam.
func (c *tcpClient) wrapErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// readAll liest alle Register und liefert die Rohbytes in derselben Reihenfolge.
// Direkt aneinanderliegende Register gleichen Typs werden in einer Anfrage
// gelesen. Lücken werden bewusst nicht überbrückt, weil viele Geräte auf
// nicht belegte Adressen mit einer Exception antworten.
func (c *tcpClient) readAll(ctx context.Context, regs []Register) ([][]byte, error) {
	order := make([]int, len(regs))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		ra, rb := regs[a], regs[b]
		if ra.RegisterType() != rb.RegisterType() {
			if ra.RegisterType() < rb.RegisterType() {
				return -1
			}
			return 1
		}
		return int(ra.Address) - int(rb.Address)
	})

	out := make([][]byte, len(regs))
	for i := 0; i < len(order); {
		first := regs[order[i]]
		typ, start := first.RegisterType(), int(first.Address)
		end := start + int(first.Length)

		j := i + 1
		for ; j < len(order); j++ {
			r := regs[order[j]]
			rEnd := int(r.Address) + int(r.Length)
			if r.RegisterType() != typ || int(r.Address) > end || max(end, rEnd)-start > maxRegistersPerRead {
				break
			}
			end = max(end, rEnd)
		}

		if end > 0x10000 {
			return nil, fmt.Errorf("register %d+%d überschreitet Adressraum", first.Address, first.Length)
		}
		data, err := c.readRegisters(ctx, typ, uint16(start), uint16(end-start))
		if err != nil {
			return nil, fmt.Errorf("lese %s %d..%d: %w", typ, start, end-1, err)
		}
		for _, k := range order[i:j] {
			off := (int(regs[k].Address) - start) * 2
			out[k] = data[off : off+int(regs[k].Length)*2]
		}
		i = j
	}
	return out, nil
}
