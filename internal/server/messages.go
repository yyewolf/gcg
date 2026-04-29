package server

import (
	"encoding/binary"
	"math"

	"github.com/yyewolf/gcg/internal/game"
)

// binaryStateMagic is the first byte of every binary state frame. It is not a
// valid first byte of a CBOR map (which starts with 0xA0–0xBF), so the client
// can distinguish frame types without a separate framing field.
const binaryStateMagic = 0x01

// Binary state frame layout (little-endian):
//
//	[0]        uint8   magic = 0x01
//	[1..8]     int64   tick
//	[9]        uint8   tickRate
//	[10..11]   uint16  nPlanets
//	per planet (7 bytes):
//	  [+0..1]  uint16  id
//	  [+2]     uint8   owner
//	  [+3..6]  int32   ships
//	[...]
//	[n..n+1]   uint16  nFleets
//	per fleet (27 bytes):
//	  [+0..3]  uint32  id
//	  [+4]     uint8   owner
//	  [+5..6]  uint16  sourceID
//	  [+7..8]  uint16  targetID
//	  [+9..10] uint16  ships
//	  [+11..14] float32 x
//	  [+15..18] float32 y
//	  [+19..22] float32 vx
//	  [+23..26] float32 vy
func encodeBinaryState(snapshot game.Snapshot, buf []byte) []byte {
	np := len(snapshot.Planets)
	nf := len(snapshot.Fleets)
	size := 12 + 7*np + 2 + 27*nf
	if cap(buf) >= size {
		buf = buf[:size]
	} else {
		buf = make([]byte, size)
	}

	buf[0] = binaryStateMagic
	binary.LittleEndian.PutUint64(buf[1:], uint64(snapshot.Tick))
	buf[9] = uint8(snapshot.TickRate)
	binary.LittleEndian.PutUint16(buf[10:], uint16(np))

	off := 12
	for _, p := range snapshot.Planets {
		binary.LittleEndian.PutUint16(buf[off:], uint16(p.ID))
		buf[off+2] = uint8(p.Owner)
		binary.LittleEndian.PutUint32(buf[off+3:], uint32(int32(p.Ships)))
		off += 7
	}

	binary.LittleEndian.PutUint16(buf[off:], uint16(nf))
	off += 2
	for _, f := range snapshot.Fleets {
		binary.LittleEndian.PutUint32(buf[off:], uint32(f.ID))
		buf[off+4] = uint8(f.Owner)
		binary.LittleEndian.PutUint16(buf[off+5:], uint16(f.SourceID))
		binary.LittleEndian.PutUint16(buf[off+7:], uint16(f.TargetID))
		binary.LittleEndian.PutUint16(buf[off+9:], uint16(f.Ships))
		binary.LittleEndian.PutUint32(buf[off+11:], math.Float32bits(float32(f.X)))
		binary.LittleEndian.PutUint32(buf[off+15:], math.Float32bits(float32(f.Y)))
		binary.LittleEndian.PutUint32(buf[off+19:], math.Float32bits(float32(f.VX)))
		binary.LittleEndian.PutUint32(buf[off+23:], math.Float32bits(float32(f.VY)))
		off += 27
	}

	return buf
}
