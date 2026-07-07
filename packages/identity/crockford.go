package identity

import "fmt"

// crockfordAlphabet is the 32-character encoding alphabet used by TypeID.
// It is Crockford base32: decimal digits followed by lowercase letters,
// with i, l, o, and u removed to avoid visual confusion with 1, 1, 0, v.
// See: https://www.crockford.com/base32.html
const crockfordAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// crockfordDecodeMap maps every byte value to its 5-bit Crockford index.
// 0xFF marks an invalid character.
var crockfordDecodeMap [256]byte

func init() {
	for i := range crockfordDecodeMap {
		crockfordDecodeMap[i] = 0xFF
	}
	for i, c := range crockfordAlphabet {
		crockfordDecodeMap[c] = byte(i)
		// Accept uppercase input; Crockford base32 is case-insensitive on read.
		if c >= 'a' && c <= 'z' {
			crockfordDecodeMap[c-32] = byte(i)
		}
	}
}

// crockfordEncode encodes 16 UUID bytes into a 26-character Crockford base32 string.
//
// The 128-bit value is treated as a big-endian integer. Two zero bits are prepended
// to give 130 bits (26 × 5), then the bits are packed into 26 five-bit groups, each
// mapped to a character in crockfordAlphabet. The first character is therefore always
// in [0-7] since the two leading padding bits are zero.
func crockfordEncode(b [16]byte) string {
	var dst [26]byte
	e := crockfordAlphabet
	dst[0] = e[(b[0]&0xE0)>>5]
	dst[1] = e[b[0]&0x1F]
	dst[2] = e[(b[1]&0xF8)>>3]
	dst[3] = e[((b[1]&0x07)<<2)|((b[2]&0xC0)>>6)]
	dst[4] = e[(b[2]&0x3E)>>1]
	dst[5] = e[((b[2]&0x01)<<4)|((b[3]&0xF0)>>4)]
	dst[6] = e[((b[3]&0x0F)<<1)|((b[4]&0x80)>>7)]
	dst[7] = e[(b[4]&0x7C)>>2]
	dst[8] = e[((b[4]&0x03)<<3)|((b[5]&0xE0)>>5)]
	dst[9] = e[b[5]&0x1F]
	dst[10] = e[(b[6]&0xF8)>>3]
	dst[11] = e[((b[6]&0x07)<<2)|((b[7]&0xC0)>>6)]
	dst[12] = e[(b[7]&0x3E)>>1]
	dst[13] = e[((b[7]&0x01)<<4)|((b[8]&0xF0)>>4)]
	dst[14] = e[((b[8]&0x0F)<<1)|((b[9]&0x80)>>7)]
	dst[15] = e[(b[9]&0x7C)>>2]
	dst[16] = e[((b[9]&0x03)<<3)|((b[10]&0xE0)>>5)]
	dst[17] = e[b[10]&0x1F]
	dst[18] = e[(b[11]&0xF8)>>3]
	dst[19] = e[((b[11]&0x07)<<2)|((b[12]&0xC0)>>6)]
	dst[20] = e[(b[12]&0x3E)>>1]
	dst[21] = e[((b[12]&0x01)<<4)|((b[13]&0xF0)>>4)]
	dst[22] = e[((b[13]&0x0F)<<1)|((b[14]&0x80)>>7)]
	dst[23] = e[(b[14]&0x7C)>>2]
	dst[24] = e[((b[14]&0x03)<<3)|((b[15]&0xE0)>>5)]
	dst[25] = e[b[15]&0x1F]
	return string(dst[:])
}

// crockfordDecode decodes a 26-character Crockford base32 string into 16 bytes.
//
// Returns an error if:
//   - the string is not exactly 26 characters
//   - any character is not in the Crockford alphabet (case-insensitive)
//   - the first character exceeds '7' (would overflow a 128-bit value)
func crockfordDecode(s string) ([16]byte, error) {
	if len(s) != 26 {
		return [16]byte{}, fmt.Errorf("identity: Crockford suffix must be 26 characters, got %d", len(s))
	}
	// The first character can only encode 3 bits (the 128-bit value padded to 130 bits
	// always has the two leading bits zero), so it must be in ['0','7'] (values 0–7).
	if s[0] > '7' {
		return [16]byte{}, fmt.Errorf("identity: TypeID overflow: first character %q exceeds '7'", s[0])
	}
	var v [26]byte
	for i := range v {
		d := crockfordDecodeMap[s[i]]
		if d == 0xFF {
			return [16]byte{}, fmt.Errorf("identity: invalid Crockford base32 character %q at position %d", s[i], i)
		}
		v[i] = d
	}
	// Unpack 26 five-bit groups back into 16 bytes.
	// Each line mirrors the corresponding encode lines in crockfordEncode.
	var b [16]byte
	b[0] = (v[0] << 5) | v[1]
	b[1] = (v[2] << 3) | (v[3] >> 2)
	b[2] = (v[3] << 6) | (v[4] << 1) | (v[5] >> 4)
	b[3] = (v[5] << 4) | (v[6] >> 1)
	b[4] = (v[6] << 7) | (v[7] << 2) | (v[8] >> 3)
	b[5] = (v[8] << 5) | v[9]
	b[6] = (v[10] << 3) | (v[11] >> 2)
	b[7] = (v[11] << 6) | (v[12] << 1) | (v[13] >> 4)
	b[8] = (v[13] << 4) | (v[14] >> 1)
	b[9] = (v[14] << 7) | (v[15] << 2) | (v[16] >> 3)
	b[10] = (v[16] << 5) | v[17]
	b[11] = (v[18] << 3) | (v[19] >> 2)
	b[12] = (v[19] << 6) | (v[20] << 1) | (v[21] >> 4)
	b[13] = (v[21] << 4) | (v[22] >> 1)
	b[14] = (v[22] << 7) | (v[23] << 2) | (v[24] >> 3)
	b[15] = (v[24] << 5) | v[25]
	return b, nil
}
