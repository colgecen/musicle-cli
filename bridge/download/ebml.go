package download

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// EBML element IDs (32-bit with VINT encoding).
const (
	EBMLID          uint32 = 0x1A45DFA3
	SegmentID       uint32 = 0x18538067
	InfoID          uint32 = 0x1549A966
	TracksID        uint32 = 0x1654AE6B
	ClusterID       uint32 = 0x1F43B675
	SeekHeadID      uint32 = 0x114D9B74
	CuesID          uint32 = 0x1C53BB6B
	TagsID          uint32 = 0x1254C367
)

// EBML info children.
const (
	TimecodeScaleID uint32 = 0x2AD7B1
	DurationID      uint32 = 0x4489
)

// Track entry children.
const (
	TrackEntryID    uint32 = 0xAE
	TrackNumberID   uint32 = 0xD7
	TrackTypeID     uint32 = 0x83
	CodecIDID       uint32 = 0x86
	AudioID         uint32 = 0xE1
	SamplingFreqID  uint32 = 0xB5
	ChannelsID      uint32 = 0x9F
)

// Cluster children.
const (
	ClusterTimecodeID uint32 = 0xE7
	SimpleBlockID     uint32 = 0xA3
	BlockGroupID      uint32 = 0xA0
	BlockID           uint32 = 0xA1
	BlockDurationID   uint32 = 0x9B
	ReferenceBlockID  uint32 = 0xFB
)

// Track types.
const (
	TrackTypeVideo    = 1
	TrackTypeAudio    = 2
	TrackTypeComplex  = 3
	TrackTypeSubtitle = 16
)

// EBMLAudioTrack holds parsed audio track info.
type EBMLAudioTrack struct {
	TrackNumber    int
	CodecID        string // "A_OPUS", "A_VORBIS", etc.
	SampleRate     float64
	Channels       int
}

// EBMLInfo holds parsed segment info.
type EBMLInfo struct {
	TimecodeScale int64 // nanoseconds per tick
	Duration      float64
}

// EBMLCluster holds timecode and block data.
type EBMLCluster struct {
	Timecode      int64
	SimpleBlocks  []EBMLSimpleBlock
}

// EBMLSimpleBlock holds a single audio/video frame (or the first frame when laced).
type EBMLSimpleBlock struct {
	TrackNumber int
	Timecode    int16 // relative to cluster timecode
	Keyframe    bool
	Data        []byte
	ExtraFrames [][]byte // additional frames from lacing
}

// vintSize returns the encoded size of a VINT given its first byte.
func vintSize(first byte) int {
	for i := 0; i < 8; i++ {
		if first&(0x80>>i) != 0 {
			return i + 1
		}
	}
	return 8
}

// readVINT reads a variable-length integer from the reader.
// Returns the decoded value and the number of bytes consumed.
func readVINT(r io.Reader) (uint64, int, error) {
	var first [1]byte
	if _, err := io.ReadFull(r, first[:]); err != nil {
		return 0, 0, err
	}
	size := vintSize(first[0])
	if size == 1 {
		return uint64(first[0] & 0x7F), 1, nil
	}
	raw := make([]byte, size)
	raw[0] = first[0]
	if _, err := io.ReadFull(r, raw[1:]); err != nil {
		return 0, 0, err
	}
	var val uint64
	for i := 0; i < size; i++ {
		val = (val << 8) | uint64(raw[i])
	}
	// Clear the VINT marker bit (most significant bit of first byte)
	marker := byte(0x80 >> (size - 1))
	val &^= uint64(marker) << (8 * (size - 1))
	return val, size, nil
}

// readSignedVINT reads a VINT and interprets it as a signed integer.
// In EBML lacing, signed values encode the sign in bit 0:
// even = positive (value/2), odd = negative (-(value+1)/2).
func readSignedVINT(r io.Reader) (int64, int, error) {
	val, n, err := readVINT(r)
	if err != nil {
		return 0, 0, err
	}
	if val&1 == 0 {
		return int64(val / 2), n, nil
	}
	return -int64((val + 1) / 2), n, nil
}

// readElementID reads a variable-length element ID from the reader.
// EBML element IDs include the VINT marker bit as part of the value.
func readElementID(r io.Reader) (uint32, int, error) {
	var first [1]byte
	if _, err := io.ReadFull(r, first[:]); err != nil {
		return 0, 0, err
	}
	size := vintSize(first[0])
	raw := make([]byte, size)
	raw[0] = first[0]
	if size > 1 {
		if _, err := io.ReadFull(r, raw[1:]); err != nil {
			return 0, 0, err
		}
	}
	var val uint64
	for i := 0; i < size; i++ {
		val = (val << 8) | uint64(raw[i])
	}
	return uint32(val), size, nil
}

// readElementSize reads a variable-length data size from the reader.
func readElementSize(r io.Reader) (uint64, int, error) {
	return readVINT(r)
}

// EBMLReader reads and navigates EBML/WebM structures.
type EBMLReader struct {
	data    []byte
	offset  int
}

// NewEBMLReader creates a new EBML reader from raw WebM bytes.
func NewEBMLReader(data []byte) *EBMLReader {
	return &EBMLReader{data: data}
}

// readUint reads an unsigned integer of n bytes from the data.
func (r *EBMLReader) readUint(n int) (uint64, error) {
	if r.offset+n > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	var val uint64
	for i := 0; i < n; i++ {
		val = (val << 8) | uint64(r.data[r.offset+i])
	}
	r.offset += n
	return val, nil
}

// readFloat reads a float of size bytes (4 or 8).
func (r *EBMLReader) readFloat(size int) (float64, error) {
	if r.offset+size > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	var val float64
	switch size {
	case 4:
		bits := binary.BigEndian.Uint32(r.data[r.offset:])
		val = float64(math.Float32frombits(bits))
		r.offset += 4
	case 8:
		bits := binary.BigEndian.Uint64(r.data[r.offset:])
		val = math.Float64frombits(bits)
		r.offset += 8
	default:
		return 0, fmt.Errorf("unsupported float size: %d", size)
	}
	return val, nil
}

// readString reads a string of the given length.
func (r *EBMLReader) readString(size int) (string, error) {
	if r.offset+size > len(r.data) {
		return "", io.ErrUnexpectedEOF
	}
	s := string(r.data[r.offset : r.offset+size])
	r.offset += size
	return s, nil
}

// skip advances the offset by n bytes.
func (r *EBMLReader) skip(n int) {
	r.offset += n
}

// ParseAudioTracks parses the Tracks element and returns audio track info.
func (r *EBMLReader) ParseAudioTracks() ([]EBMLAudioTrack, error) {
	if r.offset >= len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}

	br := bytes.NewReader(r.data[r.offset:])
	id, idLen, err := readElementID(br)
	if err != nil {
		return nil, fmt.Errorf("read tracks id: %w", err)
	}
	if id != TracksID {
		return nil, fmt.Errorf("expected Tracks element (0x%X), got 0x%X", TracksID, id)
	}
	size, sizeLen, err := readElementSize(br)
	if err != nil {
		return nil, fmt.Errorf("read tracks size: %w", err)
	}

	headerLen := idLen + sizeLen
	r.offset += headerLen

	return r.parseTracksBody(int(size))
}

func (r *EBMLReader) parseTracksBody(size int) ([]EBMLAudioTrack, error) {
	end := r.offset + size
	var tracks []EBMLAudioTrack

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return tracks, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return tracks, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		if id == TrackEntryID {
			r.offset += totalHeader
			track, err := r.parseTrackEntry(dataSize)
			if err != nil {
				return tracks, err
			}
			if track != nil {
				tracks = append(tracks, *track)
			}
		} else {
			r.offset += totalHeader + dataSize
		}
	}
	return tracks, nil
}

func (r *EBMLReader) parseTrackEntry(size int) (*EBMLAudioTrack, error) {
	end := r.offset + size
	var track EBMLAudioTrack
	var isAudio bool

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return nil, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return nil, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case TrackNumberID:
			r.offset += totalHeader
			val, _ := r.readUint(dataSize)
			track.TrackNumber = int(val)
		case TrackTypeID:
			r.offset += totalHeader
			val, _ := r.readUint(dataSize)
			isAudio = (int(val) == TrackTypeAudio)
		case CodecIDID:
			r.offset += totalHeader
			codec, _ := r.readString(dataSize)
			track.CodecID = codec
		case AudioID:
			r.offset += totalHeader
			r.parseAudioSettings(&track, dataSize)
		default:
			r.offset += totalHeader + dataSize
			continue
		}
	}

	if !isAudio {
		return nil, nil
	}
	return &track, nil
}

func (r *EBMLReader) parseAudioSettings(track *EBMLAudioTrack, size int) {
	end := r.offset + size
	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, _ := readElementID(br)
		elemSize, sizeLen, _ := readElementSize(br)
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case SamplingFreqID:
			r.offset += totalHeader
			if dataSize == 4 {
				bits := binary.BigEndian.Uint32(r.data[r.offset:])
				track.SampleRate = float64(math.Float32frombits(bits))
				r.offset += dataSize
			} else if dataSize == 8 {
				bits := binary.BigEndian.Uint64(r.data[r.offset:])
				track.SampleRate = math.Float64frombits(bits)
				r.offset += dataSize
			} else {
				r.offset += dataSize
			}
		case ChannelsID:
			r.offset += totalHeader
			val, _ := r.readUint(dataSize)
			track.Channels = int(val)
		default:
			r.offset += totalHeader + dataSize
		}
	}
}

// ParseSegmentInfo parses the Info element for timecode scale and duration.
func (r *EBMLReader) ParseSegmentInfo(size int) (*EBMLInfo, error) {
	end := r.offset + size
	info := &EBMLInfo{TimecodeScale: 1000000} // default 1ms

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return info, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return info, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case TimecodeScaleID:
			r.offset += totalHeader
			val, _ := r.readUint(dataSize)
			info.TimecodeScale = int64(val)
		case DurationID:
			r.offset += totalHeader
			if dataSize == 4 || dataSize == 8 {
				if r.offset+dataSize > len(r.data) {
					r.offset += dataSize
					break
				}
				if dataSize == 4 {
					bits := binary.BigEndian.Uint32(r.data[r.offset:])
					info.Duration = float64(math.Float32frombits(bits))
				} else {
					bits := binary.BigEndian.Uint64(r.data[r.offset:])
					info.Duration = math.Float64frombits(bits)
				}
				r.offset += dataSize
			} else {
				r.offset += dataSize
			}
		default:
			r.offset += totalHeader + dataSize
		}
	}
	return info, nil
}

// ParseClusters parses all Cluster elements and returns their blocks.
func (r *EBMLReader) ParseClusters() ([]EBMLCluster, error) {
	var clusters []EBMLCluster

	for r.offset < len(r.data) {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			break
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			break
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)
		nextOffset := r.offset + totalHeader + dataSize

		if id == ClusterID {
			r.offset += totalHeader
			cluster, err := r.parseCluster(dataSize)
			if err != nil {
				return clusters, err
			}
			clusters = append(clusters, *cluster)
		} else {
			r.offset = nextOffset
		}
	}
	return clusters, nil
}

func (r *EBMLReader) parseCluster(size int) (*EBMLCluster, error) {
	end := r.offset + size
	cluster := &EBMLCluster{}

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return cluster, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return cluster, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case ClusterTimecodeID:
			r.offset += totalHeader
			val, _ := r.readUint(dataSize)
			cluster.Timecode = int64(val)
		case SimpleBlockID:
			r.offset += totalHeader
			sb, err := r.parseSimpleBlock(dataSize)
			if err != nil {
				return cluster, err
			}
			cluster.SimpleBlocks = append(cluster.SimpleBlocks, *sb)
		default:
			r.offset += totalHeader + dataSize
		}
	}
	return cluster, nil
}

// WebMParseResult holds the parsed WebM header information.
type WebMParseResult struct {
	Info   *EBMLInfo
	Tracks []EBMLAudioTrack
}

// ParseWebM parses a complete WebM file and returns the header info and audio tracks.
// It skips the EBML header, finds the Segment, and extracts Info + Tracks.
func ParseWebM(data []byte) (*WebMParseResult, error) {
	r := NewEBMLReader(data)

	// --- EBML header ---
	br := bytes.NewReader(r.data)
	ebmlID, ebmlIDLen, err := readElementID(br)
	if err != nil {
		return nil, fmt.Errorf("read EBML ID: %w", err)
	}
	if ebmlID != EBMLID {
		return nil, fmt.Errorf("not EBML: expected 0x%X, got 0x%X", EBMLID, ebmlID)
	}
	ebmlSize, ebmlSizeLen, err := readElementSize(br)
	if err != nil {
		return nil, fmt.Errorf("read EBML size: %w", err)
	}
	// Skip past the entire EBML header (ID + size field + data)
	r.offset = ebmlIDLen + ebmlSizeLen + int(ebmlSize)

	// --- Segment ---
	br2 := bytes.NewReader(r.data[r.offset:])
	segID, segIDLen, err := readElementID(br2)
	if err != nil {
		return nil, fmt.Errorf("read Segment ID: %w", err)
	}
	if segID != SegmentID {
		return nil, fmt.Errorf("expected Segment (0x%X), got 0x%X", SegmentID, segID)
	}
	segSize, segSizeLen, err := readElementSize(br2)
	if err != nil {
		return nil, fmt.Errorf("read Segment size: %w", err)
	}
	r.offset += segIDLen + segSizeLen
	segEnd := r.offset + int(segSize)

	// Scan Segment children for Info and Tracks
	result := &WebMParseResult{}
	for r.offset < segEnd {
		br3 := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br3)
		if err != nil {
			break
		}
		elemSize, sizeLen, err := readElementSize(br3)
		if err != nil {
			break
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case InfoID:
			r.offset += totalHeader
			info, err := r.ParseSegmentInfo(dataSize)
			if err != nil {
				return nil, fmt.Errorf("parse Info: %w", err)
			}
			result.Info = info
		case TracksID:
			r.offset += totalHeader
			tracks, err := r.parseTracksBody(dataSize)
			if err != nil {
				return nil, fmt.Errorf("parse Tracks: %w", err)
			}
			result.Tracks = tracks
		default:
			r.offset += totalHeader + dataSize
		}

		// Early exit if we have both Info and Tracks
		if result.Info != nil && len(result.Tracks) > 0 {
			break
		}
	}

	if result.Info == nil {
		return nil, fmt.Errorf("Info element not found")
	}
	if len(result.Tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks found")
	}
	return result, nil
}

// AudioFrame represents a single decoded audio frame with timing.
type AudioFrame struct {
	TimecodeNs int64 // absolute timecode in nanoseconds
	Data       []byte
}

// ExtractAudioFrames parses the entire WebM and extracts audio frames for the
// first audio track found. Returns frames with absolute nanosecond timecodes.
func ExtractAudioFrames(data []byte) (frames []AudioFrame, info *EBMLInfo, track *EBMLAudioTrack, err error) {
	pr, err := ParseWebM(data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse webm: %w", err)
	}

	// Find first audio track
	if len(pr.Tracks) == 0 {
		return nil, nil, nil, fmt.Errorf("no audio tracks in WebM")
	}
	audioTrack := pr.Tracks[0]

	timecodeScale := pr.Info.TimecodeScale
	if timecodeScale <= 0 {
		timecodeScale = 1000000 // default 1ms
	}

	r := NewEBMLReader(data)

	// Skip to after EBML header + Segment header
	br := bytes.NewReader(r.data)
	_, ebmlIDLen, _ := readElementID(br)
	ebmlSize, ebmlSizeLen, _ := readElementSize(br)
	ebmlHeaderEnd := ebmlIDLen + ebmlSizeLen + int(ebmlSize)

	br2 := bytes.NewReader(r.data[ebmlHeaderEnd:])
	_, segIDLen, _ := readElementID(br2)
	segSize, segSizeLen, _ := readElementSize(br2)
	segEnd := ebmlHeaderEnd + segIDLen + segSizeLen + int(segSize)

	r.offset = ebmlHeaderEnd + segIDLen + segSizeLen

	// Scan for Clusters and collect frames
	for r.offset < segEnd {
		br3 := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br3)
		if err != nil {
			break
		}
		elemSize, sizeLen, err := readElementSize(br3)
		if err != nil {
			break
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		if id == ClusterID {
			r.offset += totalHeader
			cluster, err := r.parseCluster(dataSize)
			if err != nil {
				return frames, pr.Info, &audioTrack, fmt.Errorf("parse cluster: %w", err)
			}
			for _, sb := range cluster.SimpleBlocks {
				if sb.TrackNumber != audioTrack.TrackNumber {
					continue
				}
				absTimecode := (cluster.Timecode + int64(sb.Timecode)) * timecodeScale
				frames = append(frames, AudioFrame{
					TimecodeNs: absTimecode,
					Data:       sb.Data,
				})
				for _, extra := range sb.ExtraFrames {
					frames = append(frames, AudioFrame{
						TimecodeNs: absTimecode,
						Data:       extra,
					})
				}
			}
		} else {
			r.offset += totalHeader + dataSize
		}
	}

	if len(frames) == 0 {
		return nil, pr.Info, &audioTrack, fmt.Errorf("no audio frames found for track %d", audioTrack.TrackNumber)
	}
	return frames, pr.Info, &audioTrack, nil
}

func (r *EBMLReader) parseSimpleBlock(size int) (*EBMLSimpleBlock, error) {
	if r.offset+4 > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}

	sb := &EBMLSimpleBlock{}
	start := r.offset
	blockEnd := start + size

	// Track number (VINT)
	trackVal, trackLen, err := readVINT(bytes.NewReader(r.data[r.offset:]))
	if err != nil {
		return nil, err
	}
	sb.TrackNumber = int(trackVal)
	r.offset += trackLen

	// Timecode (int16, big-endian)
	if r.offset+2 > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	sb.Timecode = int16(binary.BigEndian.Uint16(r.data[r.offset:]))
	r.offset += 2

	// Flags: 1 byte
	if r.offset >= len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	flags := r.data[r.offset]
	r.offset++
	sb.Keyframe = (flags&0x80 != 0)

	// Remaining bytes after headers
	remaining := blockEnd - r.offset
	if remaining < 0 {
		return nil, fmt.Errorf("invalid simpleblock size")
	}
	if r.offset+remaining > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}

	lacingType := (flags >> 1) & 0x03
	if lacingType == 0 {
		// No lacing: all remaining data is one frame
		// Some encoders prepend a 2-byte BE size prefix to the Opus data.
		// If the first 2 bytes equal (remaining - 2), strip them.
		if remaining >= 4 {
			prefixLen := int(binary.BigEndian.Uint16(r.data[r.offset:]))
			if prefixLen == remaining-2 {
				r.offset += 2
				remaining -= 2
			}
		}
		sb.Data = make([]byte, remaining)
		copy(sb.Data, r.data[r.offset:r.offset+remaining])
		r.offset += remaining
		return sb, nil
	}

	// Lacing: first byte after flags is (numFrames - 1)
	if remaining < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	numFrames := int(r.data[r.offset]) + 1
	r.offset++

	frameSizes := make([]int, numFrames)
	remaining -= 1 // consumed the frame count byte

	switch lacingType {
	case 1: // Xiph lacing
		for i := 0; i < numFrames-1; i++ {
			var sz int
			for {
				if r.offset >= blockEnd {
					return nil, fmt.Errorf("xiph lacing overflow")
				}
				b := r.data[r.offset]
				r.offset++
				remaining--
				sz += int(b)
				if b != 255 {
					break
				}
			}
			frameSizes[i] = sz
		}
		frameSizes[numFrames-1] = remaining

	case 2: // Fixed lacing
		if numFrames == 0 {
			return nil, fmt.Errorf("fixed lacing: no frames")
		}
		frameSize := remaining / numFrames
		for i := 0; i < numFrames; i++ {
			frameSizes[i] = frameSize
		}

	case 3: // EBML lacing
		// First frame size as unsigned VINT
		if remaining < 1 {
			return nil, io.ErrUnexpectedEOF
		}
		vintReader := bytes.NewReader(r.data[r.offset:])
		v, vintLen, err := readVINT(vintReader)
		if err != nil {
			return nil, fmt.Errorf("ebml lacing first size: %w", err)
		}
		frameSizes[0] = int(v)
		r.offset += vintLen
		remaining -= vintLen

		// Subsequent frame sizes as signed VINT differences
		for i := 1; i < numFrames-1; i++ {
			if remaining < 1 {
				return nil, io.ErrUnexpectedEOF
			}
			vintReader.Reset(r.data[r.offset:])
			signedV, svintLen, err := readSignedVINT(vintReader)
			if err != nil {
				return nil, fmt.Errorf("ebml lacing diff: %w", err)
			}
			r.offset += svintLen
			remaining -= svintLen
			frameSizes[i] = frameSizes[i-1] + int(signedV)
		}
		// Last frame = remaining data
		frameSizes[numFrames-1] = remaining

	default:
		return nil, fmt.Errorf("unknown lacing type %d", lacingType)
	}

	// Extract all frames
	for i := 0; i < numFrames; i++ {
		if frameSizes[i] > remaining || frameSizes[i] < 0 {
			return nil, fmt.Errorf("lacing frame %d size %d exceeds remaining %d", i, frameSizes[i], remaining)
		}
		frameData := make([]byte, frameSizes[i])
		copy(frameData, r.data[r.offset:r.offset+frameSizes[i]])
		r.offset += frameSizes[i]
		remaining -= frameSizes[i]

		if i == 0 {
			sb.Data = frameData
		} else {
			sb.ExtraFrames = append(sb.ExtraFrames, frameData)
		}
	}

	return sb, nil
}


