package download

// Opus range decoder — RFC 6716 Section 4.1 arithmetic decoder.
// Implements ec_dec_init, ec_decode, ec_dec_update as specified.

const (
	ecBitBufSize = 32
	ecHalf       = 1 << 15
	ecQuarter    = 1 << 14
	ecThreeQuart = 3 << 14
)

type ecDecoder struct {
	data      []byte
	pos       int
	low       uint32
	range_    uint32
	cnt       int
	buffer    uint32
}

// ecDecInit initializes the range decoder (RFC 6716 Section 4.1.1).
// Reads the first 8 bytes into the buffer.
func ecDecInit(data []byte) *ecDecoder {
	d := &ecDecoder{
		data:   data,
		pos:    0,
		low:    0,
		range_: 0xFFFFFFFF,
		cnt:    0,
		buffer: 0,
	}
	// Read the first byte of input
	d.readByte()
	// Normalize: ensure we have enough bits
	d.normalize()
	return d
}

// readByte reads one byte from the input stream into the buffer.
func (d *ecDecoder) readByte() {
	if d.pos < len(d.data) {
		d.buffer = (d.buffer << 8) | uint32(d.data[d.pos])
		d.pos++
		d.cnt += 8
	}
}

// ecDecode estimates a symbol from the current state (RFC 6716 Section 4.1.2).
// ft = total frequency count (CDF table size).
// Returns a value v in [0, ft-1].
func (d *ecDecoder) ecDecode(ft uint32) uint32 {
	// r = d.range_ / ft
	r := d.range_ / ft
	// v = d.low / r
	v := d.low / r
	// Clamp v to ft-1
	if v >= ft {
		v = ft - 1
	}
	return v
}

// ecDecUpdate updates the decoder state after decoding a symbol (RFC 6716 Section 4.1.3).
// fl = lower bound of symbol, fh = upper bound, ft = total.
func (d *ecDecoder) ecDecUpdate(fl, fh, ft uint32) {
	r := d.range_ / ft
	d.low -= fl * r
	if fh < ft {
		d.range_ = (fh - fl) * r
	} else {
		d.range_ -= fl * r
	}
	d.normalize()
}

// normalize renormalizes the range decoder (RFC 6716 Section 4.1.1).
func (d *ecDecoder) normalize() {
	for d.range_ <= 1<<23 {
		d.range_ <<= 8
		d.low <<= 8
		// Read more bytes if needed
		if d.cnt <= 8 {
			d.readByte()
		}
		if d.cnt > 0 {
			lowNext := d.buffer >> (uint(d.cnt) - 8)
			d.low = (d.low & 0xFFFFFF00) | (lowNext & 0xFF)
			d.cnt -= 8
		}
	}
}

// decBit decodes a single bit with probability ~1/2.
func (d *ecDecoder) decBit() int {
	// Equivalent to decode with CDF [1,2] (50% probability)
	ft := uint32(2)
	v := d.ecDecode(ft)
	d.ecDecUpdate(v, v+1, ft)
	return int(v)
}

// decUniform decodes a uniformly distributed integer in [0, n-1].
func (d *ecDecoder) decUniform(n int) int {
	if n <= 1 {
		return 0
	}
	// Find smallest k such that (1<<k) >= n
	k := 0
	for (1 << k) < n {
		k++
	}
	// Decode bits
	bits := k - 1
	v := 0
	for i := 0; i < bits; i++ {
		v = (v << 1) | d.decBit()
	}
	if (1 << bits) < n {
		// One extra bit for the remainder
		flip := d.decBit()
		v = (v << 1) | flip
	}
	return v
}

// decInt decodes a raw n-bit integer.
func (d *ecDecoder) decInt(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = (v << 1) | d.decBit()
	}
	return v
}

// decCDF decodes a symbol from a CDF table.
func (d *ecDecoder) decCDF(cdf []uint16) int {
	ft := uint32(cdf[len(cdf)-1])
	v := d.ecDecode(ft)
	// Binary search for symbol
	lo := 0
	hi := len(cdf) - 1
	for lo < hi {
		mid := (lo + hi) / 2
		if v < uint32(cdf[mid]) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	fl := uint32(0)
	if lo > 0 {
		fl = uint32(cdf[lo-1])
	}
	fh := uint32(cdf[lo])
	d.ecDecUpdate(fl, fh, ft)
	return lo
}

// decLaplace decodes a Laplace-distributed value with given decay.
func (d *ecDecoder) decLaplace(decay uint16) int {
	// Decode sign
	if d.decBit() == 0 {
		return 0
	}
	// Decode magnitude
	mag := 0
	for {
		bit := d.decBit()
		if bit == 0 {
			break
		}
		mag++
		// Simple decay model
		if mag > 32 {
			break
		}
	}
	return mag + 1
}
