package download

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MP4 atom types (32-bit FourCC).
type MP4Atom struct {
	Type   [4]byte
	Size   uint32 // atom size including header (0 means extends to EOF)
	Header int    // bytes consumed by header (8 or 16 for large size)
	Data   []byte // raw data (for leaf atoms)
}

// sampleEntry holds codec configuration for an audio track.
type MP4AudioSampleEntry struct {
	SampleRate  float64
	Channels    int
	SampleSize  int // bits per sample
	CodecName   string
	ExtraData   []byte // codec-specific (e.g. ESDS for AAC)
}

// MP4SampleTable holds the sample table for a track.
type MP4SampleTable struct {
	SampleSizes []uint32       // stsz: size of each sample
	ChunkOffsets []uint64      // stco/co64: file offset of each chunk
	SamplesPerChunk []uint32  // stsc: samples per chunk (per entry)
	FirstChunk   []uint32     // stsc: first chunk for each entry
	TimeToSample []MP4TimeEntry // stts: duration per sample
	SampleCount  uint32
}

// MP4TimeEntry maps sample count to duration.
type MP4TimeEntry struct {
	Count    uint32
	Duration uint32
}

// readAtomHeader reads the header of an MP4 atom.
// Returns the atom type as string, data size, and total header size.
func readAtomHeader(data []byte, offset int) (string, uint64, int, error) {
	if offset+8 > len(data) {
		return "", 0, 0, io.ErrUnexpectedEOF
	}
	size := binary.BigEndian.Uint32(data[offset : offset+4])
	atomType := string(data[offset+4 : offset+8])

	headerSize := 8
	var dataSize uint64

	if size == 1 {
		// Extended size (8 more bytes)
		if offset+16 > len(data) {
			return "", 0, 0, io.ErrUnexpectedEOF
		}
		dataSize = binary.BigEndian.Uint64(data[offset+8 : offset+16])
		headerSize = 16
	} else if size == 0 {
		// Rest of file
		dataSize = uint64(len(data) - offset)
	} else {
		dataSize = uint64(size)
	}

	return atomType, dataSize, headerSize, nil
}

// MP4Reader reads and navigates MP4 atoms.
type MP4Reader struct {
	data   []byte
	offset int
}

// NewMP4Reader creates a new MP4 reader.
func NewMP4Reader(data []byte) *MP4Reader {
	return &MP4Reader{data: data, offset: 0}
}

// NextAtom reads the next atom at the current offset.
// Returns nil when exhausted.
func (r *MP4Reader) NextAtom() (*MP4Atom, error) {
	if r.offset >= len(r.data) {
		return nil, nil
	}

	atomType, dataSize, headerSize, err := readAtomHeader(r.data, r.offset)
	if err != nil {
		return nil, err
	}

	if dataSize < uint64(headerSize) {
		return nil, fmt.Errorf("invalid atom size: %d < header %d", dataSize, headerSize)
	}

	atom := &MP4Atom{
		Size:   uint32(dataSize),
		Header: headerSize,
		Data:   r.data[r.offset+headerSize : r.offset+int(dataSize)],
	}
	copy(atom.Type[:], atomType)

	r.offset += int(dataSize)
	return atom, nil
}

// FindFirstAtom finds the first occurrence of an atom with the given type.
func (r *MP4Reader) FindFirstAtom(atomType string) *MP4Atom {
	saved := r.offset
	r.offset = 0
	defer func() { r.offset = saved }()

	for {
		atom, err := r.NextAtom()
		if err != nil || atom == nil {
			break
		}
		if string(atom.Type[:]) == atomType {
			return atom
		}
	}
	return nil
}

// ParseAudioSampleTable parses the stbl (sample table) atom and returns the
// sample table for finding and reading AAC frames.
func ParseAudioSampleTable(data []byte) (*MP4SampleTable, *MP4AudioSampleEntry, error) {
	r := NewMP4Reader(data)
	stbl := &MP4SampleTable{}
	var entry *MP4AudioSampleEntry

	for {
		atom, err := r.NextAtom()
		if err != nil || atom == nil {
			break
		}
		switch string(atom.Type[:]) {
		case "stsd":
			entry = parseSTSD(atom.Data)
		case "stts":
			stbl.TimeToSample = parseSTTS(atom.Data)
		case "stsc":
			stbl.FirstChunk, stbl.SamplesPerChunk = parseSTSC(atom.Data)
		case "stsz":
			stbl.SampleSizes, stbl.SampleCount = parseSTSZ(atom.Data)
		case "stco":
			stbl.ChunkOffsets = parseSTCO(atom.Data)
		case "co64":
			stbl.ChunkOffsets = parseCO64(atom.Data)
		}
	}
	return stbl, entry, nil
}

// parseSTSD parses the Sample Description atom.
func parseSTSD(data []byte) *MP4AudioSampleEntry {
	if len(data) < 8 {
		return nil
	}
	// version(1) + flags(3) + entryCount(4)
	entryCount := binary.BigEndian.Uint32(data[4:8])
	if entryCount == 0 {
		return nil
	}
	offset := 8
	for i := uint32(0); i < entryCount && offset+8 < len(data); i++ {
		size := binary.BigEndian.Uint32(data[offset : offset+4])
		codec := string(data[offset+4 : offset+8])
		entry := &MP4AudioSampleEntry{CodecName: codec}

		if codec == "mp4a" || codec == "drms" {
			// Skip: reserved(6) + dataRefIdx(2) + version(2) + revision(2) + vendor(4)
			// + channels(2) + sampleSize(2) + compressionId(2) + packetSize(2)
			// + sampleRate(4: fixed point 16.16)
			base := offset + 8
			if base+20 > len(data) {
				break
			}
			entry.Channels = int(binary.BigEndian.Uint16(data[base+8 : base+10]))
			entry.SampleSize = int(binary.BigEndian.Uint16(data[base+10 : base+12]))
			srFixed := binary.BigEndian.Uint32(data[base+16 : base+20])
			entry.SampleRate = float64(srFixed >> 16)

			// ESDS atom inside (codec-specific data)
			esdsOffset := base + 20
			for esdsOffset+8 < offset+int(size) {
				esdsSize := binary.BigEndian.Uint32(data[esdsOffset : esdsOffset+4])
				esdsType := string(data[esdsOffset+4 : esdsOffset+8])
				if esdsType == "esds" {
					entry.ExtraData = parseESDS(data[esdsOffset+8:esdsOffset+int(esdsSize)])
					break
				}
				esdsOffset += int(esdsSize)
			}
		}
		return entry
	}
	return nil
}

// parseESDS extracts the AAC decoder configuration from an ESDS atom.
func parseESDS(data []byte) []byte {
	if len(data) < 5 {
		return nil
	}
	// version(1) + flags(3) + ESDescrTag(1)
	offset := 4
	if offset >= len(data) || data[offset] != 0x03 {
		return nil
	}
	offset++
	// ES_ID (variable length, skip)
	if offset >= len(data) {
		return nil
	}
	esDescLen := int(data[offset])
	offset++
	if esDescLen > len(data)-offset {
		esDescLen = len(data) - offset
	}
	offset += esDescLen
	// DecoderConfigDescrTag (0x04)
	if offset >= len(data) || data[offset] != 0x04 {
		return nil
	}
	offset++
	if offset >= len(data) {
		return nil
	}
	dcLen := int(data[offset])
	offset++
	if dcLen > len(data)-offset {
		dcLen = len(data) - offset
	}
	// Decoder config descriptor: objectTypeIndication(1) + streamType(1) + ... + DecSpecificInfoTag(0x05)
	dcEnd := offset + dcLen
	offset += 2 // skip objectType + streamType
	if offset+3 > dcEnd {
		return nil
	}
	bufSize := int(uint32(data[offset])<<16 | uint32(data[offset+1])<<8 | uint32(data[offset+2]))
	_ = bufSize
	offset += 3
	// maxBitrate(4) + avgBitrate(4)
	offset += 8
	// DecSpecificInfoTag (0x05)
	if offset >= dcEnd || data[offset] != 0x05 {
		return nil
	}
	offset++
	if offset >= dcEnd {
		return nil
	}
	dsiLen := int(data[offset])
	offset++
	if offset+dsiLen > dcEnd {
		dsiLen = dcEnd - offset
	}
	if dsiLen > 0 {
		dsi := make([]byte, dsiLen)
		copy(dsi, data[offset:offset+dsiLen])
		return dsi
	}
	return nil
}

// parseSTTS parses Time-to-Sample atom.
func parseSTTS(data []byte) []MP4TimeEntry {
	if len(data) < 8 {
		return nil
	}
	// version(1) + flags(3) + entryCount(4)
	count := binary.BigEndian.Uint32(data[4:8])
	entries := make([]MP4TimeEntry, 0, count)
	offset := 8
	for i := uint32(0); i < count && offset+8 <= len(data); i++ {
		entries = append(entries, MP4TimeEntry{
			Count:    binary.BigEndian.Uint32(data[offset : offset+4]),
			Duration: binary.BigEndian.Uint32(data[offset+4 : offset+8]),
		})
		offset += 8
	}
	return entries
}

// parseSTSC parses Sample-to-Chunk atom.
func parseSTSC(data []byte) (firstChunk []uint32, samplesPerChunk []uint32) {
	if len(data) < 8 {
		return nil, nil
	}
	count := binary.BigEndian.Uint32(data[4:8])
	offset := 8
	for i := uint32(0); i < count && offset+12 <= len(data); i++ {
		firstChunk = append(firstChunk, binary.BigEndian.Uint32(data[offset:offset+4]))
		samplesPerChunk = append(samplesPerChunk, binary.BigEndian.Uint32(data[offset+4:offset+8]))
		offset += 12
	}
	return
}

// parseSTSZ parses Sample Size atom.
func parseSTSZ(data []byte) (sizes []uint32, sampleCount uint32) {
	if len(data) < 12 {
		return nil, 0
	}
	// version(1) + flags(3) + sampleSize(4)
	sampleSize := binary.BigEndian.Uint32(data[4:8])
	count := binary.BigEndian.Uint32(data[8:12])

	if sampleSize == 0 {
		// Variable sample sizes
		if len(data) < int(12+count*4) {
			return nil, count
		}
		sizes = make([]uint32, count)
		for i := uint32(0); i < count; i++ {
			sizes[i] = binary.BigEndian.Uint32(data[12+i*4 : 16+i*4])
		}
	} else {
		// Constant sample size
		sizes = make([]uint32, count)
		for i := uint32(0); i < count; i++ {
			sizes[i] = sampleSize
		}
	}
	return sizes, count
}

// parseSTCO parses Chunk Offset atom (32-bit offsets).
func parseSTCO(data []byte) []uint64 {
	if len(data) < 8 {
		return nil
	}
	count := binary.BigEndian.Uint32(data[4:8])
	offsets := make([]uint64, 0, count)
	offset := 8
	for i := uint32(0); i < count && offset+4 <= len(data); i++ {
		offsets = append(offsets, uint64(binary.BigEndian.Uint32(data[offset:offset+4])))
		offset += 4
	}
	return offsets
}

// parseCO64 parses Chunk Large Offset atom (64-bit offsets).
func parseCO64(data []byte) []uint64 {
	if len(data) < 8 {
		return nil
	}
	count := binary.BigEndian.Uint32(data[4:8])
	offsets := make([]uint64, 0, count)
	offset := 8
	for i := uint32(0); i < count && offset+8 <= len(data); i++ {
		offsets = append(offsets, binary.BigEndian.Uint64(data[offset:offset+8]))
		offset += 8
	}
	return offsets
}

// ExtractAACFrames uses the sample table to extract individual AAC frames from
// the mdat (media data) section.
func ExtractAACFrames(mdat []byte, stbl *MP4SampleTable) ([][]byte, error) {
	return extractFramesFromTable(mdat, stbl)
}

func extractFramesFromTable(mdat []byte, stbl *MP4SampleTable) ([][]byte, error) {
	if len(stbl.ChunkOffsets) == 0 || len(stbl.SampleSizes) == 0 {
		return nil, fmt.Errorf("empty sample table")
	}

	frames := make([][]byte, len(stbl.SampleSizes))
	sampleIdx := uint32(0)

	for ci, chunkOffset := range stbl.ChunkOffsets {
		// Determine samples in this chunk
		samplesHere := uint32(0)
		for j := len(stbl.FirstChunk) - 1; j >= 0; j-- {
			if stbl.FirstChunk[j] <= uint32(ci+1) {
				samplesHere = stbl.SamplesPerChunk[j]
				break
			}
		}
		if samplesHere == 0 {
			samplesHere = 1
		}

		offset := int64(chunkOffset)
		for s := uint32(0); s < samplesHere && sampleIdx < uint32(len(stbl.SampleSizes)); s++ {
			size := int64(stbl.SampleSizes[sampleIdx])
			if offset+size > int64(len(mdat)) {
				return nil, fmt.Errorf("sample %d past mdat boundary", sampleIdx)
			}
			frame := make([]byte, size)
			copy(frame, mdat[offset:offset+size])
			frames[sampleIdx] = frame
			offset += size
			sampleIdx++
		}
	}

	return frames[:sampleIdx], nil
}

// DurationFromSTTS calculates total duration in timebase units from stts entries.
func DurationFromSTTS(entries []MP4TimeEntry) uint64 {
	var total uint64
	for _, e := range entries {
		total += uint64(e.Count) * uint64(e.Duration)
	}
	return total
}
