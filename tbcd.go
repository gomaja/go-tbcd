// Package tbcd encodes and decodes TBCD digit strings: the TBCD-STRING of
// 3GPP TS 29.002 (MAP), used for IMSI, IMEI, IMEISV and the digits of an
// AddressString, and the semi-octet representation of 3GPP TS 23.040 (SMS),
// used for the address fields of a TPDU and for the SC address.
//
// The governing text of TS 29.002 is V19.1.0 §17.7.8 (MAP-CommonDataTypes):
//
//	TBCD-STRING ::= OCTET STRING
//	-- This type (Telephony Binary Coded Decimal String) is used to
//	-- represent several digits from 0 through 9, *, #, a, b, c, two
//	-- digits per octet, each digit encoded 0000 to 1001 (0 to 9),
//	-- 1010 (*), 1011 (#), 1100 (a), 1101 (b) or 1110 (c); 1111 used
//	-- as filler when there is an odd number of digits.
//
//	-- bits 8765 of octet n encoding digit 2n
//	-- bits 4321 of octet n encoding digit 2(n-1) +1
//
// TS 23.040 V20.0.0 §9.1.2.3 packs decimal digits into semi-octets in the
// same order, the earlier digit in bits 0 to 3 (bits 4321 above). After an
// odd number of digits, "the bits with bit numbers 4 to 7 within the last
// octet are fill bits and shall always be set to "1111"". A receiver shows
// the semi-octets 1010 to 1110 as *, #, a, b and c, the number digits of the
// called party BCD number, TS 24.008 V20.1.0 §10.5.4.7 Table 10.5.118, which
// also has 1111 as the end mark after an odd number of digits.
//
// TBCD is therefore not hexadecimal: the alphabet is exactly
// "0123456789*#abc" and the nibble value 1111 is a filler, never a digit.
//
// The two specifications differ on a filler that a digit follows. TS 29.002
// has a filler only after an odd number of digits, and a MAP receiver should
// not guess at a string that breaks that rule, so by default the decoders
// reject it. TS 23.040 §9.1.2.3 has an SMS receiver skip it: "If a mobile
// receives "1111" in a position prior to the last semi-octet then processing
// shall commence with the next semi-octet and the intervening semi-octet
// shall be ignored." The SkipInteriorFill option selects that behaviour.
//
// Decode decodes a whole TBCD-STRING. DecodeDigits decodes a known number of
// digits, as a TS 23.040 address field is decoded from its Address-Length.
// AppendEncode and AppendDecode append to a caller's buffer and allocate only
// to grow it.
package tbcd

import (
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrInvalidCharacter is returned by Encode and AppendEncode for a
	// character outside the TBCD alphabet "0123456789*#abc".
	ErrInvalidCharacter = errors.New("tbcd: character outside the TBCD alphabet 0-9 * # a b c")

	// ErrMisplacedFiller is returned when a filler nibble (1111) stands
	// where a digit is required: by Decode and AppendDecode when a digit
	// follows a filler, and by DecodeDigits when one of the digits it is to
	// return is a filler. SkipInteriorFill skips such a filler instead.
	ErrMisplacedFiller = errors.New("tbcd: misplaced filler nibble")

	// ErrShortInput is returned by DecodeDigits when the input holds fewer
	// digits than asked for: it has fewer octets than the digits need, or,
	// with SkipInteriorFill, the fillers skipped leave too few digits in
	// those octets.
	ErrShortInput = errors.New("tbcd: input too short for the digit count")

	// ErrNegativeCount is returned by DecodeDigits for a negative digit
	// count.
	ErrNegativeCount = errors.New("tbcd: negative digit count")
)

const (
	filler = 0x0F
	// alphabet maps a nibble value 0..14 to its digit. Value 15 is the filler
	// and has no digit.
	alphabet = "0123456789*#abc"
	// stackDigits is the number of digits Decode and DecodeDigits collect on
	// the stack before they allocate, so that the returned string is their
	// only allocation for a field of up to 32 octets. That covers the
	// TBCD-STRING types of TS 29.002 (at most 8 octets), the digits of an
	// AddressString (at most 19 octets: maxAddressLength is 20, with the
	// type-of-address octet) and the TS 23.040 addresses (at most 10 octets).
	stackDigits = 64
)

// A DecodeOption changes a rule of Decode, AppendDecode and DecodeDigits. The
// zero DecodeOption changes nothing.
type DecodeOption struct {
	skipInteriorFill bool
}

// SkipInteriorFill makes Decode, AppendDecode and DecodeDigits skip a filler
// nibble (1111) where their default rules require a digit, instead of
// returning ErrMisplacedFiller, as 3GPP TS 23.040 V20.0.0 §9.1.2.3 has a
// receiver do.
func SkipInteriorFill() DecodeOption {
	return DecodeOption{skipInteriorFill: true}
}

func skipInteriorFill(opts []DecodeOption) bool {
	for _, o := range opts {
		if o.skipInteriorFill {
			return true
		}
	}
	return false
}

// Encode returns the TBCD coding of s. s must consist only of characters from
// the alphabet "0123456789*#abc"; any other character (including A-F, d, e, f
// and non-ASCII characters) yields an error wrapping ErrInvalidCharacter that
// names the character and its byte index in s, and a nil slice. Bits 4321 of
// each octet hold the earlier digit and bits 8765 the later one. An odd number
// of digits is completed with the filler nibble 1111 in bits 8765 of the last
// octet. The empty string encodes to an empty, non-nil slice.
func Encode(s string) ([]byte, error) {
	// A non-nil dst, so that "" encodes to an empty, non-nil slice.
	// AppendEncode checks s before it allocates.
	b, err := AppendEncode([]byte{}, s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// AppendEncode appends the TBCD coding of s to dst and returns the extended
// slice, with the rules of Encode. It allocates only when dst lacks the
// capacity for the (len(s)+1)/2 octets. On error it returns dst as it was
// passed in and has written nothing to it.
func AppendEncode(dst []byte, s string) ([]byte, error) {
	for i, r := range s {
		if r > 0x7F {
			return dst, fmt.Errorf("%w: %q at index %d", ErrInvalidCharacter, r, i)
		}
		if _, ok := nibble(byte(r)); !ok {
			return dst, fmt.Errorf("%w: %q at index %d", ErrInvalidCharacter, r, i)
		}
	}
	dst = slices.Grow(dst, len(s)/2+len(s)%2) // (len(s)+1)/2, without overflow
	for i := 0; i+1 < len(s); i += 2 {
		lo, _ := nibble(s[i])
		hi, _ := nibble(s[i+1])
		dst = append(dst, hi<<4|lo)
	}
	if len(s)%2 == 1 {
		lo, _ := nibble(s[len(s)-1])
		dst = append(dst, filler<<4|lo)
	}
	return dst, nil
}

func nibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c == '*':
		return 0xA, true
	case c == '#':
		return 0xB, true
	case c >= 'a' && c <= 'c':
		return c - 'a' + 0xC, true
	}
	return 0, false
}

// Decode returns the digits of the TBCD-STRING b. Bits 4321 of each octet
// hold the earlier digit.
//
// Filler rules. The nibble 1111 is a filler. By default a filler is valid
// only after the last digit; a filler followed by a digit anywhere, including
// a 1111 in bits 4321 of an octet whose bits 8765 hold a digit, returns an
// error wrapping ErrMisplacedFiller that names the octet of that digit. With
// SkipInteriorFill a filler anywhere is skipped instead, and Decode returns no
// error.
//
// Trailing filler octets are tolerated and dropped. TBCD-STRING only has a
// filler for an odd digit count, but a fixed-size field must carry filler
// octets: GroupId is a TBCD-STRING (SIZE (3)) for which "the TBCD filler
// (1111) is used to fill unused half octets" when it has fewer than six
// digits, and which "shall be filled with six TBCD fillers (1111)" in
// VoiceGroupCallData and VoiceBroadcastData when a longGroupId is present
// (TS 29.002 §17.7.1). Some implementations also pad other fields with
// 0xFF. Accepting the filler octets loses no information, since a nibble that
// follows a filler can only be another filler. The consequence is that
// Decode is not injective: Encode(Decode(x)) equals x with its trailing 0xFF
// octets removed, which is the canonical form.
//
// Decode(nil) and Decode([]byte{}) both return ("", nil). A zero-digit
// string is a well-formed TBCD-STRING; whether an empty value is acceptable
// is a SIZE or presence rule of the containing type and is for the caller to
// enforce. A nil slice carries no more information than an empty one in Go.
func Decode(b []byte, opts ...DecodeOption) (string, error) {
	var buf [stackDigits]byte
	d, err := AppendDecode(buf[:0], b, opts...)
	if err != nil {
		return "", err
	}
	return string(d), nil
}

// AppendDecode appends the digits of the TBCD-STRING b to dst and returns the
// extended slice, with the rules of Decode. It allocates only when dst lacks
// the capacity for the digits. On error it returns dst as it was passed in
// and has written nothing to it, not even beyond its length. dst and b must
// not overlap.
func AppendDecode(dst, b []byte, opts ...DecodeOption) ([]byte, error) {
	n, err := countDigits(b, skipInteriorFill(opts))
	if err != nil {
		return dst, err
	}
	dst = slices.Grow(dst, n)
	// After countDigits a filler can only be skipped: with the default rules
	// none has a digit after it.
	for _, o := range b {
		if lo := o & 0x0F; lo != filler {
			dst = append(dst, alphabet[lo])
		}
		if hi := o >> 4; hi != filler {
			dst = append(dst, alphabet[hi])
		}
	}
	return dst, nil
}

// countDigits returns the number of digits in b, or the error that Decode
// returns for b.
func countDigits(b []byte, skip bool) (int, error) {
	n := 0
	seenFiller := false
	for i, o := range b {
		for _, d := range [2]byte{o & 0x0F, o >> 4} {
			if d == filler {
				seenFiller = true
				continue
			}
			if seenFiller && !skip {
				return 0, fmt.Errorf("%w: octet %d (0x%02x) has a digit after a filler", ErrMisplacedFiller, i, o)
			}
			n++
		}
	}
	return n, nil
}

// DecodeDigits returns n digits decoded from the first (n+1)/2 octets of b,
// as a 3GPP TS 23.040 address field is decoded from its Address-Length, the
// number of useful semi-octets (§9.1.2.5). Octets of b beyond those are not
// read.
//
// For an odd n, bits 8765 of the last octet read hold the filler a sender
// sets to 1111. By default that half-octet carries no digit and is not
// checked, and a filler among the n digits returns an error wrapping
// ErrMisplacedFiller that names its octet. With SkipInteriorFill such a
// filler is skipped instead, as TS 23.040 §9.1.2.3 has a receiver do, and the
// half-octets that follow it, the last one included, complete the n digits; a
// half-octet left over after the n-th digit is still not checked. If the
// fillers skipped leave fewer than n digits in the (n+1)/2 octets,
// DecodeDigits returns an error wrapping ErrShortInput.
//
// If b has fewer than (n+1)/2 octets, DecodeDigits returns an error wrapping
// ErrShortInput, and for a negative n one wrapping ErrNegativeCount.
// DecodeDigits(b, 0) returns ("", nil) for any b.
func DecodeDigits(b []byte, n int, opts ...DecodeOption) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("%w: %d", ErrNegativeCount, n)
	}
	octets := n/2 + n%2 // (n+1)/2, without overflow for the largest n
	if len(b) < octets {
		return "", fmt.Errorf("%w: %d digits need %d octets, have %d", ErrShortInput, n, octets, len(b))
	}
	skip := skipInteriorFill(opts)
	var buf [stackDigits]byte
	out := buf[:0]
	if n > len(buf) {
		out = make([]byte, 0, n) // n is at most 2*len(b)
	}
	for i, o := range b[:octets] {
		for _, d := range [2]byte{o & 0x0F, o >> 4} {
			if len(out) == n {
				break // the last half-octet of an odd n
			}
			if d == filler {
				if !skip {
					return "", fmt.Errorf("%w: octet %d (0x%02x) has a filler as digit %d of %d", ErrMisplacedFiller, i, o, len(out)+1, n)
				}
				continue
			}
			out = append(out, alphabet[d])
		}
	}
	if len(out) < n {
		return "", fmt.Errorf("%w: %d octets hold %d of %d digits after skipping fillers", ErrShortInput, octets, len(out), n)
	}
	return string(out), nil
}
