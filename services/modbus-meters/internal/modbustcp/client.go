// Package modbustcp ist ein minimaler Modbus-TCP-Client, der nur
// Holding- und Input-Register liest.
package modbustcp

import (
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"time"

	"github.com/KlinkerD/Ems/contracts/modbus"
)

// functionCode liefert den Modbus-Funktionscode zum Lesen von t. Registertypen
// und das Lese-Limit (modbus.MaxRegistersPerRead) kommen aus dem Vertrag, damit
// sie nur an einer Stelle definiert sind.
func functionCode(t modbus.RegisterType) (byte, error) {
	switch t {
	case modbus.HoldingRegister:
		return 0x03, nil
	case modbus.InputRegister:
		return 0x04, nil
	default:
		return 0, fmt.Errorf("unbekannter registerType %q", t)
	}
}

// Block ist ein zusammenhängender Bereich von Count Registern ab Address.
type Block struct {
	Type    modbus.RegisterType
	Address uint16
	Count   uint16
}

// Exception ist eine Fehlerantwort des Geräts, z. B. Code 2 für ungültige Adresse.
type Exception struct {
	Function byte
	Code     byte
}

// Error implementiert das error-Interface.
func (e Exception) Error() string {
	return fmt.Sprintf("modbus exception 0x%02x bei Funktion 0x%02x", e.Code, e.Function)
}

// Client ist eine Modbus-TCP-Verbindung zu einem Gerät.
// Er ist nicht für parallele Nutzung gedacht.
type Client struct {
	conn    net.Conn
	unitID  uint8
	timeout time.Duration
	txID    uint16
}

// Dial verbindet mit addr. timeout gilt für den Verbindungsaufbau und
// für jede einzelne Anfrage.
func Dial(ctx context.Context, addr string, unitID uint8, timeout time.Duration) (*Client, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, unitID: unitID, timeout: timeout}, nil
}

// Close schließt die Verbindung.
func (c *Client) Close() error {
	return c.conn.Close()
}

// readRegisters liest count Register ab addr und liefert die Rohbytes.
// Nach einem Fehler ist der Zustand der Verbindung unklar, sie sollte
// dann geschlossen werden.
func (c *Client) readRegisters(ctx context.Context, typ modbus.RegisterType, addr, count uint16) ([]byte, error) {
	fc, err := functionCode(typ)
	if err != nil {
		return nil, err
	}
	if count == 0 || count > modbus.MaxRegistersPerRead {
		return nil, fmt.Errorf("count %d außerhalb 1..%d", count, modbus.MaxRegistersPerRead)
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
		return nil, Exception{Function: fc, Code: pdu[1]}
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
func (c *Client) wrapErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// ReadAll liest alle Blöcke und liefert die Rohbytes in derselben Reihenfolge.
// Direkt aneinanderliegende Blöcke gleichen Typs werden in einer Anfrage
// gelesen. Lücken werden bewusst nicht überbrückt, weil viele Geräte auf
// nicht belegte Adressen mit einer Exception antworten.
func (c *Client) ReadAll(ctx context.Context, blocks []Block) ([][]byte, error) {
	order := make([]int, len(blocks))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		ba, bb := blocks[a], blocks[b]
		return cmp.Or(cmp.Compare(ba.Type, bb.Type), cmp.Compare(ba.Address, bb.Address))
	})

	out := make([][]byte, len(blocks))
	for i := 0; i < len(order); {
		first := blocks[order[i]]
		typ, start := first.Type, int(first.Address)
		end := start + int(first.Count)

		j := i + 1
		for ; j < len(order); j++ {
			b := blocks[order[j]]
			bEnd := int(b.Address) + int(b.Count)
			if b.Type != typ || int(b.Address) > end || max(end, bEnd)-start > modbus.MaxRegistersPerRead {
				break
			}
			end = max(end, bEnd)
		}

		if end > 0x10000 {
			return nil, fmt.Errorf("register %d+%d überschreitet Adressraum", first.Address, first.Count)
		}
		data, err := c.readRegisters(ctx, typ, uint16(start), uint16(end-start))
		if err != nil {
			return nil, fmt.Errorf("lese %s %d..%d: %w", typ, start, end-1, err)
		}
		for _, k := range order[i:j] {
			off := (int(blocks[k].Address) - start) * 2
			out[k] = data[off : off+int(blocks[k].Count)*2]
		}
		i = j
	}
	return out, nil
}
