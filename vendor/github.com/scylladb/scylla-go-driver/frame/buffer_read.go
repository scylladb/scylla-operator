package frame

import (
	"fmt"
	"log"
)

// All the read functions call readByte or readInto as they would want to read a single byte or copy a slice of bytes.
// The functions handle the unlikely situation of read error.
// Read errors can are caused by reading over the buffer size.

func (b *Buffer) Error() error {
	return b.readErr
}

func (b *Buffer) readByte() Byte {
	if b.readErr != nil {
		return 0
	}

	p, err := b.buf.ReadByte()
	if err != nil {
		b.readErr = fmt.Errorf("buffer readByte error: %w", err)
	}
	return p
}

func (b *Buffer) readInto(p []byte) {
	if b.readErr != nil {
		return
	}

	l, err := b.buf.Read(p)
	if err != nil {
		b.readErr = fmt.Errorf("buffer Read error: %w", err)
	}
	if l != len(p) {
		b.readErr = fmt.Errorf("unexpected end of buffer")
	}
}

const maxBytesSize int = 128_000_000

// readCopy reads next n bytes from the buffer, and returns a copy.
func (b *Buffer) readCopy(n int) Bytes {
	// Here n can be corrupted, so we have to check for that before allocating
	// potentially enormously big slice.
	if b.readErr != nil {
		return nil
	}
	if n < 0 || maxBytesSize < n {
		b.readErr = fmt.Errorf("too big Bytes size: %v", n)
		return nil
	}

	p := make(Bytes, n)
	b.readInto(p)
	return p
}

func (b *Buffer) ReadByte() Byte {
	return b.readByte()
}

func (b *Buffer) ReadShort() Short {
	return Short(b.readByte())<<8 | Short(b.readByte())
}

func (b *Buffer) ReadStreamID() StreamID {
	return StreamID(b.readByte())<<8 | StreamID(b.readByte())
}

func (b *Buffer) ReadInt() Int {
	a := [4]byte{0, 0, 0, 0}
	b.readInto(a[:])
	return Int(a[0])<<24 |
		Int(a[1])<<16 |
		Int(a[2])<<8 |
		Int(a[3])
}

func (b *Buffer) ReadLong() Long {
	a := [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
	b.readInto(a[:])
	return Long(a[0])<<56 |
		Long(a[1])<<48 |
		Long(a[2])<<40 |
		Long(a[3])<<32 |
		Long(a[4])<<24 |
		Long(a[5])<<16 |
		Long(a[6])<<8 |
		Long(a[7])
}

func (b *Buffer) ReadOpCode() OpCode {
	o := b.readByte()
	if Debug {
		if o > OpAuthSuccess {
			log.Printf("unknown operation code: %v", o)
		}
	}
	return o
}

func (b *Buffer) ReadUUID() UUID {
	var u UUID
	b.readInto(u[:])
	return u
}

func (b *Buffer) ReadHeaderFlags() QueryFlags {
	return b.readByte()
}

func (b *Buffer) ReadQueryFlags() QueryFlags {
	return b.readByte()
}

func (b *Buffer) ReadResultFlags() ResultFlags {
	return b.ReadInt()
}

func (b *Buffer) ReadPreparedFlags() PreparedFlags {
	return b.ReadInt()
}

func (b *Buffer) ReadBytes() Bytes {
	n := b.ReadInt()
	if n < 0 {
		return nil
	}
	return b.readCopy(int(n))
}

func (b *Buffer) ReadShortBytes() ShortBytes {
	return b.readCopy(int(b.ReadShort()))
}

// Length equal to -1 represents null.
// Length equal to -2 represents not set.
func (b *Buffer) ReadValue() Value {
	n := b.ReadInt()
	if Debug {
		if n < -2 {
			log.Printf("unknown value length")
		}
	}

	v := Value{N: n}
	if n > 0 {
		v.Bytes = b.readCopy(int(n))
	}
	return v
}

func (b *Buffer) ReadInet() Inet {
	n := b.readByte()
	if Debug {
		if n != 4 && n != 16 {
			log.Printf("unknown ip length")
		}
	}
	return Inet{IP: b.readCopy(int(n)), Port: b.ReadInt()}
}

func (b *Buffer) ReadString() string {
	return string(b.readCopy(int(b.ReadShort())))
}

func (b *Buffer) ReadLongString() string {
	return string(b.readCopy(int(b.ReadInt())))
}

func (b *Buffer) ReadStringList() StringList {
	// Read length of the string list.
	n := b.ReadShort()
	l := make(StringList, 0, n)
	for i := Short(0); i < n; i++ {
		// Read the strings and append them to the list.
		l = append(l, b.ReadString())
	}
	return l
}

func (b *Buffer) ReadStringMap() StringMap {
	// Read the number of elements in the map.
	n := b.ReadShort()
	m := make(StringMap, n)
	for i := Short(0); i < n; i++ {
		k := b.ReadString()
		v := b.ReadString()
		m[k] = v
	}
	return m
}

func (b *Buffer) ReadStringMultiMap() StringMultiMap {
	// Read the number of elements in the map.
	n := b.ReadShort()
	m := make(StringMultiMap, n)
	for i := Short(0); i < n; i++ {
		k := b.ReadString()
		v := b.ReadStringList()
		m[k] = v
	}
	return m
}

func (b *Buffer) ReadBytesMap() BytesMap {
	n := b.ReadShort()
	m := make(BytesMap, n)
	for i := Short(0); i < n; i++ {
		k := b.ReadString()
		v := b.ReadBytes()
		m[k] = v
	}
	return m
}

func (b *Buffer) ReadStartupOptions() StartupOptions {
	return b.ReadStringMap()
}

func contains(l StringList, s string) bool {
	for _, k := range l {
		if s == k {
			return true
		}
	}
	return false
}

func (b *Buffer) WriteStartupOptions(m StartupOptions) {
	count := 0
	if Debug {
		for k, v := range mandatoryOptions {
			if s, ok := m[k]; !(ok && contains(v, s)) {
				log.Printf("unknown mandatory Startup option %s: %s", k, s)
			}
			count++
		}
		for k, v := range possibleOptions {
			if s, ok := m[k]; ok && !contains(v, s) {
				log.Printf("unknown Startup option %s: %s", k, s)
			} else if ok {
				count++
			}
		}
		if count != len(m) {
			log.Printf("unknown Startup option")
		}
	}
	b.WriteStringMap(m)
}

func (b *Buffer) ReadTopologyChangeType() TopologyChangeType {
	t := TopologyChangeType(b.ReadString())
	if Debug {
		if _, ok := allTopologyChangeTypes[t]; !ok {
			log.Printf("unknown TopologyChangeType: %s", t)
		}
	}
	return t
}

func (b *Buffer) ReadStatusChangeType() StatusChangeType {
	t := StatusChangeType(b.ReadString())
	if Debug {
		if _, ok := allStatusChangeTypes[t]; !ok {
			log.Printf("unknown StatusChangeType: %s", t)
		}
	}
	return t
}

func (b *Buffer) ReadSchemaChangeType() SchemaChangeType {
	t := SchemaChangeType(b.ReadString())
	if Debug {
		if _, ok := allSchemaChangeTypes[t]; !ok {
			log.Printf("unknown SchemaChangeType: %s", t)
		}
	}
	return t
}

// allation is not required. It is done inside SchemaChange event.
func (b *Buffer) ReadSchemaChangeTarget() SchemaChangeTarget {
	v := SchemaChangeTarget(b.ReadString())
	if Debug {
		if _, ok := allSchemaChangeTargets[v]; !ok {
			log.Printf("unknown SchemaChangeTarget: %s", v)
		}
	}
	return v
}

func (b *Buffer) ReadErrorCode() ErrorCode {
	return b.ReadInt()
}

func (b *Buffer) ReadConsistency() Consistency {
	v := b.ReadShort()
	if Debug && v > LOCALONE {
		log.Printf("unknown consistency: %v", v)
	}
	return v
}

func (b *Buffer) ReadWriteType() WriteType {
	w := WriteType(b.ReadString())
	if Debug {
		if _, ok := allWriteTypes[w]; !ok {
			log.Printf("unknown write type: %s", w)
		}
	}
	return w
}

func (b *Buffer) ReadCustomOption() *CustomOption {
	return &CustomOption{
		Name: b.ReadString(),
	}
}

func (b *Buffer) ReadListOption() *ListOption {
	return &ListOption{
		Element: b.ReadOption(),
	}
}

func (b *Buffer) ReadMapOption() *MapOption {
	return &MapOption{
		Key:   b.ReadOption(),
		Value: b.ReadOption(),
	}
}

func (b *Buffer) ReadSetOption() *SetOption {
	return &SetOption{
		Element: b.ReadOption(),
	}
}

func (b *Buffer) ReadUDTOption() *UDTOption {
	ks := b.ReadString()
	name := b.ReadString()
	n := b.ReadShort()
	fn := make(StringList, n)
	ft := make(OptionList, n)

	for i := range fn {
		fn[i] = b.ReadString()
		ft[i] = b.ReadOption()
	}

	return &UDTOption{
		Keyspace:   ks,
		Name:       name,
		fieldNames: fn,
		fieldTypes: ft,
	}
}

func (b *Buffer) ReadTupleOption() *TupleOption {
	return &TupleOption{
		ValueTypes: b.ReadOptionList(),
	}
}

func (b *Buffer) ReadOptionList() OptionList {
	n := b.ReadShort()
	ol := make(OptionList, n)
	for i := range ol {
		ol[i] = b.ReadOption()
	}
	return ol
}

func (b *Buffer) ReadOption() Option {
	id := OptionID(b.ReadShort())
	switch id {
	case CustomID:
		return Option{
			ID:     id,
			Custom: b.ReadCustomOption(),
		}
	case ListID:
		return Option{
			ID:   id,
			List: b.ReadListOption(),
		}
	case MapID:
		return Option{
			ID:  id,
			Map: b.ReadMapOption(),
		}
	case SetID:
		return Option{
			ID:  id,
			Set: b.ReadSetOption(),
		}
	case UDTID:
		return Option{
			ID:  id,
			UDT: b.ReadUDTOption(),
		}
	case TupleID:
		return Option{
			ID:    id,
			Tuple: b.ReadTupleOption(),
		}
	default:
		if Debug {
			if id < ASCIIID || DurationID < id {
				log.Printf("unknown Option ID: %d", id)
			}
		}
		return Option{
			ID: id,
		}
	}
}

func (b *Buffer) ReadColumnSpec(f ResultFlags) ColumnSpec {
	if f&GlobalTablesSpec == 0 {
		return ColumnSpec{
			Keyspace: b.ReadString(),
			Table:    b.ReadString(),
			Name:     b.ReadString(),
			Type:     b.ReadOption(),
		}
	}

	return ColumnSpec{
		Name: b.ReadString(),
		Type: b.ReadOption(),
	}
}

const maxColumnSpecSliceSize = 1_230_770 // 1230770 is 128MB divided by ColumnSpec size.

func (b *Buffer) ReadResultMetadata() ResultMetadata {
	r := ResultMetadata{
		Flags:      b.ReadResultFlags(),
		ColumnsCnt: b.ReadInt(),
	}

	if r.Flags&HasMorePages != 0 {
		r.PagingState = b.ReadBytes()
	}

	if r.Flags&NoMetadata != 0 {
		return r
	}

	if r.Flags&GlobalTablesSpec != 0 {
		r.GlobalKeyspace = b.ReadString()
		r.GlobalTable = b.ReadString()
	}

	if r.ColumnsCnt < 0 || maxColumnSpecSliceSize < r.ColumnsCnt {
		b.readErr = fmt.Errorf("too big ColumnSpec slice size: %v", r.ColumnsCnt)
		return ResultMetadata{}
	}
	r.Columns = make([]ColumnSpec, r.ColumnsCnt)
	for i := range r.Columns {
		r.Columns[i] = b.ReadColumnSpec(r.Flags)
	}

	return r
}

const maxShortSliceSize = 64_000_000

func (b *Buffer) ReadPreparedMetadata() PreparedMetadata {
	p := PreparedMetadata{
		Flags:      b.ReadPreparedFlags(),
		ColumnsCnt: b.ReadInt(),
		PkCnt:      b.ReadInt(),
	}

	if p.PkCnt < 0 || maxShortSliceSize < p.PkCnt {
		b.readErr = fmt.Errorf("too big Short slice size: %v", p.PkCnt)
		return PreparedMetadata{}
	}
	p.PkIndexes = make([]Short, p.PkCnt)
	for i := range p.PkIndexes {
		p.PkIndexes[i] = b.ReadShort()
	}

	if p.Flags&GlobalTablesSpec != 0 {
		p.GlobalKeyspace = b.ReadString()
		p.GlobalTable = b.ReadString()
	}

	if p.ColumnsCnt < 0 || maxColumnSpecSliceSize < p.ColumnsCnt {
		b.readErr = fmt.Errorf("too big ColumnSpec slice size: %v", p.ColumnsCnt)
		return PreparedMetadata{}
	}
	p.Columns = make([]ColumnSpec, p.ColumnsCnt)
	for i := range p.Columns {
		p.Columns[i] = b.ReadColumnSpec(p.Flags)
	}

	return p
}
