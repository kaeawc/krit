package scanner

// Framed columnar codec for fileShard payloads. Replaces gob as the
// in-pack shard encoding for v6 (issue #351 Steps 3+4):
//
// - One intra-shard name table deduplicates every identifier, kind,
//   and visibility string (gob wrote each occurrence in full and
//   re-emitted its schema prefix per shard).
// - Kind and Visibility reuse the name table rather than carrying a
//   separate enum table — the set is small (≤9 distinct values across
//   both fields) and their IDs always fit in a single varint byte.
// - File, Path, ContentHash are no longer persisted. File is constant
//   within a shard (the pack key carries the path); Path and
//   ContentHash come from the LoadShard args. A wrong-key hit is
//   already gated by the pack's 32-char hash key + per-blob CRC.
// - Version is implicit in the payload's magic+version header, not
//   stored per row.
//
// Wire format (all integers little-endian; varints unsigned unless
// marked signed via zig-zag):
//
//   magic u32   = 0x4B534843 "KSHC"
//   version u16 = 1
//   bloom: u32 length + bytes
//   nameTableCount: uvarint
//     { nameLen: uvarint; name: UTF-8 bytes } * nameTableCount
//   symbolCount: uvarint
//     { nameID, kindID, visID: uvarint
//       line, startByte, endByte: svarint
//       flags: u8 (bit0=IsOverride, bit1=IsTest, bit2=IsMain) } * symbolCount
//   refCount: uvarint
//     { nameID: uvarint; line: svarint; flags: u8 (bit0=InComment) } * refCount

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	shardPayloadMagic   uint32 = 0x4B534843 // "KSHC"
	shardPayloadVersion uint16 = 1
)

type codecWriter struct {
	buf []byte
}

func (w *codecWriter) putU8(v uint8)       { w.buf = append(w.buf, v) }
func (w *codecWriter) putU16(v uint16)     { w.buf = binary.LittleEndian.AppendUint16(w.buf, v) }
func (w *codecWriter) putU32(v uint32)     { w.buf = binary.LittleEndian.AppendUint32(w.buf, v) }
func (w *codecWriter) putUvarint(v uint64) { w.buf = binary.AppendUvarint(w.buf, v) }
func (w *codecWriter) putVarint(v int64)   { w.buf = binary.AppendVarint(w.buf, v) }
func (w *codecWriter) putBytesU32(b []byte) {
	w.putU32(uint32(len(b)))
	w.buf = append(w.buf, b...)
}
func (w *codecWriter) putBytesVar(b []byte) {
	w.putUvarint(uint64(len(b)))
	w.buf = append(w.buf, b...)
}

type codecReader struct {
	buf []byte
	pos int
}

var errShardTruncated = errors.New("shard payload: truncated")

func (r *codecReader) need(n int) error {
	if r.pos+n > len(r.buf) {
		return errShardTruncated
	}
	return nil
}
func (r *codecReader) getU8() (uint8, error) {
	if err := r.need(1); err != nil {
		return 0, err
	}
	v := r.buf[r.pos]
	r.pos++
	return v, nil
}
func (r *codecReader) getU16() (uint16, error) {
	if err := r.need(2); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v, nil
}
func (r *codecReader) getU32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, nil
}
func (r *codecReader) getUvarint() (uint64, error) {
	v, n := binary.Uvarint(r.buf[r.pos:])
	if n <= 0 {
		return 0, errShardTruncated
	}
	r.pos += n
	return v, nil
}
func (r *codecReader) getVarint() (int64, error) {
	v, n := binary.Varint(r.buf[r.pos:])
	if n <= 0 {
		return 0, errShardTruncated
	}
	r.pos += n
	return v, nil
}
func (r *codecReader) getBytesU32() ([]byte, error) {
	n, err := r.getU32()
	if err != nil {
		return nil, err
	}
	if err := r.need(int(n)); err != nil {
		return nil, err
	}
	out := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return out, nil
}
func (r *codecReader) getBytesVar() ([]byte, error) {
	n, err := r.getUvarint()
	if err != nil {
		return nil, err
	}
	if err := r.need(int(n)); err != nil {
		return nil, err
	}
	out := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return out, nil
}

// encodeShardPayload produces the uncompressed columnar byte stream
// for s. The returned slice owns its storage — safe to hand to the
// zstd encoder or the test harness.
func encodeShardPayload(s *fileShard) []byte {
	tab := make(map[string]uint32, len(s.Symbols)+len(s.References)+8)
	names := make([]string, 0, len(s.Symbols)+len(s.References)+8)
	intern := func(v string) uint32 {
		if i, ok := tab[v]; ok {
			return i
		}
		i := uint32(len(names))
		tab[v] = i
		names = append(names, v)
		return i
	}
	sNameID := make([]uint32, len(s.Symbols))
	sKindID := make([]uint32, len(s.Symbols))
	sVisID := make([]uint32, len(s.Symbols))
	for i, sym := range s.Symbols {
		sNameID[i] = intern(sym.Name)
		sKindID[i] = intern(sym.Kind)
		sVisID[i] = intern(sym.Visibility)
	}
	rNameID := make([]uint32, len(s.References))
	for i, ref := range s.References {
		rNameID[i] = intern(ref.Name)
	}

	w := &codecWriter{buf: make([]byte, 0, 64+len(s.Bloom)+16*len(s.Symbols)+6*len(s.References))}
	w.putU32(shardPayloadMagic)
	w.putU16(shardPayloadVersion)
	w.putBytesU32(s.Bloom)
	w.putUvarint(uint64(len(names)))
	for _, n := range names {
		w.putBytesVar([]byte(n))
	}
	w.putUvarint(uint64(len(s.Symbols)))
	for i, sym := range s.Symbols {
		w.putUvarint(uint64(sNameID[i]))
		w.putUvarint(uint64(sKindID[i]))
		w.putUvarint(uint64(sVisID[i]))
		w.putVarint(int64(sym.Line))
		w.putVarint(int64(sym.StartByte))
		w.putVarint(int64(sym.EndByte))
		var f uint8
		if sym.IsOverride {
			f |= 1
		}
		if sym.IsTest {
			f |= 2
		}
		if sym.IsMain {
			f |= 4
		}
		w.putU8(f)
	}
	w.putUvarint(uint64(len(s.References)))
	for i, ref := range s.References {
		w.putUvarint(uint64(rNameID[i]))
		w.putVarint(int64(ref.Line))
		var f uint8
		if ref.InComment {
			f |= 1
		}
		w.putU8(f)
	}
	return w.buf
}

// decodeShardPayload is the inverse of encodeShardPayload. The caller
// supplies path so Symbol.File / Reference.File can be re-hydrated;
// a shard's rows all share the same file by construction.
func decodeShardPayload(data []byte, path string) (*fileShard, error) {
	r := &codecReader{buf: data}
	m, err := r.getU32()
	if err != nil {
		return nil, err
	}
	if m != shardPayloadMagic {
		return nil, fmt.Errorf("shard payload: bad magic %#x", m)
	}
	v, err := r.getU16()
	if err != nil {
		return nil, err
	}
	if v != shardPayloadVersion {
		return nil, fmt.Errorf("shard payload: unsupported version %d", v)
	}
	bloomRef, err := r.getBytesU32()
	if err != nil {
		return nil, err
	}
	var bloom []byte
	if len(bloomRef) > 0 {
		bloom = make([]byte, len(bloomRef))
		copy(bloom, bloomRef)
	}
	nameCount, err := r.getUvarint()
	if err != nil {
		return nil, err
	}
	names := make([]string, nameCount)
	for i := range names {
		b, err := r.getBytesVar()
		if err != nil {
			return nil, err
		}
		names[i] = string(b)
	}
	lookup := func(id uint64) (string, error) {
		if id >= uint64(len(names)) {
			return "", fmt.Errorf("nameID %d out of range (table=%d)", id, len(names))
		}
		return names[id], nil
	}

	symCount, err := r.getUvarint()
	if err != nil {
		return nil, err
	}
	syms := make([]Symbol, symCount)
	for i := range syms {
		nid, err := r.getUvarint()
		if err != nil {
			return nil, err
		}
		kid, err := r.getUvarint()
		if err != nil {
			return nil, err
		}
		vid, err := r.getUvarint()
		if err != nil {
			return nil, err
		}
		name, err := lookup(nid)
		if err != nil {
			return nil, err
		}
		kind, err := lookup(kid)
		if err != nil {
			return nil, err
		}
		vis, err := lookup(vid)
		if err != nil {
			return nil, err
		}
		line, err := r.getVarint()
		if err != nil {
			return nil, err
		}
		sb, err := r.getVarint()
		if err != nil {
			return nil, err
		}
		eb, err := r.getVarint()
		if err != nil {
			return nil, err
		}
		f, err := r.getU8()
		if err != nil {
			return nil, err
		}
		syms[i] = Symbol{
			Name:       name,
			Kind:       kind,
			Visibility: vis,
			File:       path,
			Line:       int(line),
			StartByte:  int(sb),
			EndByte:    int(eb),
			IsOverride: f&1 != 0,
			IsTest:     f&2 != 0,
			IsMain:     f&4 != 0,
		}
	}

	refCount, err := r.getUvarint()
	if err != nil {
		return nil, err
	}
	refs := make([]Reference, refCount)
	for i := range refs {
		nid, err := r.getUvarint()
		if err != nil {
			return nil, err
		}
		name, err := lookup(nid)
		if err != nil {
			return nil, err
		}
		line, err := r.getVarint()
		if err != nil {
			return nil, err
		}
		f, err := r.getU8()
		if err != nil {
			return nil, err
		}
		refs[i] = Reference{
			Name:      name,
			File:      path,
			Line:      int(line),
			InComment: f&1 != 0,
		}
	}
	return &fileShard{
		Path:        path,
		Version:     crossFileShardVersion,
		Symbols:     syms,
		References:  refs,
		Bloom:       bloom,
	}, nil
}
