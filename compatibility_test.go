package tbcd

import (
	"bytes"
	"errors"
	"testing"
)

// Vectors from github.com/gomaja/go-sms, encoding/semioctet/semioctet_test.go,
// TestDecode; names below are the original row names. The source decoder is
// destination-limited; a zero-length destination requests no digits.
func TestSMSSemioctetDecodeVectors(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"nil", nil, ""},
		{"nil dst", []byte{0x10, 0x32, 0x54, 0x76}, ""},
		{"nil src", nil, ""},
		{"empty dst", []byte{0x10, 0x32, 0x54, 0x76}, ""},
		{"empty src", []byte{}, ""},
		{"limit src", []byte{0x10, 0x32, 0x54, 0x76}, "01234567"},
		{"fill limit src", []byte{0x10, 0x32, 0x54, 0xf6}, "0123456"},
		{"fill limit even dst", []byte{0x10, 0x32, 0x54, 0xf6, 0x98}, "012345"},
		{"no fill limit even dst", []byte{0x10, 0x32, 0x54, 0x76, 0x98}, "012345"},
		{"fill limit odd dst", []byte{0x10, 0x32, 0xf4, 0x76}, "01234"},
		{"alphabet", []byte{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe}, "0123456789*#abc"},
		// The source lists "no fill limit even dst" twice.
		{"no fill limit even dst repeated", []byte{0x10, 0x32, 0x54, 0x76, 0x98}, "012345"},
		// Source TestDecode/"no fill limit odd dst" gives ErrMissingFill.
		// Address.UnmarshalBinary in encoding/tpdu/address.go instead makes
		// room for l+1 digits, then truncates to l (TS 23.040 V20.0.0 §9.1.2.5).
		// DecodeDigits likewise ignores the high half-octet for l=5.
		{"no fill limit odd dst", []byte{0x10, 0x32, 0x54, 0x76}, "01234"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, skip := range []bool{false, true} {
				got, err := DecodeDigits(tc.raw, len(tc.want), decodeOptions(skip)...)
				if got != tc.want || err != nil {
					t.Fatalf("DecodeDigits(%x,%d,skip=%t)=%q,%v; want %q", tc.raw, len(tc.want), skip, got, err, tc.want)
				}
			}
		})
	}
	// Source TestDecode/"skip inter fill" is a whole-field/SMSC case:
	// eight digits occupy five octets. Address-Length 8 allows four octets,
	// which hold seven digits, so TPDU decoding must report underflow.
	raw := []byte{0x10, 0x32, 0xf4, 0x76, 0xf8}
	got, err := Decode(raw, SkipInteriorFill())
	if got != "01234678" || err != nil {
		t.Fatalf("whole-field interior filler: %q,%v", got, err)
	}
	for _, skip := range []bool{false, true} {
		sentinel := ErrMisplacedFiller
		if skip {
			sentinel = ErrShortInput
		}
		got, err = DecodeDigits(raw, 8, decodeOptions(skip)...)
		if got != "" || !errors.Is(err, sentinel) {
			t.Fatalf("bounded interior filler (skip=%t): %q,%v", skip, got, err)
		}
	}
	// Source FuzzDecode seed 1f 32 ff 54 gives ErrMissingFill with a
	// four-digit destination (the final octet has both digit 4 and digit 5),
	// and returns 12345 with a five-digit destination. Whole-field Decode
	// returns 12345. Neither count fits its bounded DecodeDigits window:
	// n=4 reads two octets and n=5 reads three; each holds only 123.
	raw = []byte{0x1f, 0x32, 0xff, 0x54}
	got, err = Decode(raw, SkipInteriorFill())
	if got != "12345" || err != nil {
		t.Fatalf("fuzz seed as whole field: %q,%v", got, err)
	}
	for _, n := range []int{4, 5} {
		got, err = DecodeDigits(raw, n, SkipInteriorFill())
		if got != "" || !errors.Is(err, ErrShortInput) {
			t.Fatalf("fuzz seed bounded n=%d: %q,%v", n, got, err)
		}
		checkDigitsModel(t, raw, n, false)
	}
}

// Vectors from github.com/gomaja/go-sms, encoding/semioctet/semioctet_test.go,
// TestEncode, original row names. This codec normalises nil/empty input to
// an empty non-nil result, as its public contract requires.
func TestSMSSemioctetEncodeVectors(t *testing.T) {
	for _, tc := range []struct {
		name, s string
		raw     []byte
	}{
		{"nil", "", nil},
		{"empty src", "", []byte{}},
		{"even src", "01234567", []byte{0x10, 0x32, 0x54, 0x76}},
		{"odd src", "0123456", []byte{0x10, 0x32, 0x54, 0xf6}},
		{"alphabet", "0123456789*#abc", []byte{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.s)
			if got == nil || err != nil || !bytes.Equal(got, tc.raw) {
				t.Fatalf("Encode(%q)=%x,%v; want %x", tc.s, got, err, tc.raw)
			}
		})
	}
	// TestEncode/"invalid digit".
	got, err := Encode("012345D6789")
	if got != nil || !errors.Is(err, ErrInvalidCharacter) || err.Error() != "tbcd: character outside the TBCD alphabet 0-9 * # a b c: 'D' at index 6" {
		t.Fatalf("invalid digit: %x,%v", got, err)
	}
}

type smsTPVector struct {
	name               string
	raw                []byte
	n                  int
	want               string
	skipErr, strictErr error
}

// github.com/gomaja/go-sms, encoding/tpdu/address_test.go,
// TestAddressUnmarshalBinary, original semi-octet row names. Only the digit
// field is included: Address-Length and Type-of-Address are caller framing.
var smsTPVectors = []smsTPVector{
	{"empty", nil, 0, "", nil, nil},
	{"number", []byte{0x16, 0x04, 0x89, 0x56, 0x26, 0xf9}, 11, "61409865629", nil, nil},
	{"number alphabet", []byte{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe}, 15, "0123456789*#abc", nil, nil},
	{"fill 0000", []byte{0x16, 0x04, 0x89, 0x56, 0x26, 0x09}, 11, "61409865629", nil, nil},
	{"fill 1110", []byte{0x16, 0x04, 0x89, 0x56, 0x26, 0xe9}, 11, "61409865629", nil, nil},
	{"fill 1010 of one digit", []byte{0xa7}, 1, "7", nil, nil},
	{"fill within odd number", []byte{0xf1, 0x23}, 3, "132", nil, ErrMisplacedFiller},
	{"fill within odd number underflow", []byte{0xf1, 0xf3}, 3, "", ErrShortInput, ErrMisplacedFiller},
	{"short number", []byte{0x16, 0x04, 0x89, 0x56}, 11, "", ErrShortInput, ErrShortInput},
	{"short number pad", []byte{0x16, 0x04, 0x89, 0x56, 0x97, 0xf7}, 12, "", ErrShortInput, ErrMisplacedFiller},
	{"max number", []byte{0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0x65, 0x87, 0x09}, 20, "12345678901234567890", nil, nil},
	{"fill within number", []byte{0x16, 0x04, 0x26, 0xf9, 0x89, 0x56}, 11, "61406299865", nil, ErrMisplacedFiller},
}

func TestSMSTPDUAddressVectors(t *testing.T) {
	for _, tc := range smsTPVectors {
		t.Run(tc.name, func(t *testing.T) {
			for _, skip := range []bool{false, true} {
				wantErr := tc.strictErr
				if skip {
					wantErr = tc.skipErr
				}
				want := tc.want
				if wantErr != nil {
					want = ""
				}
				got, err := DecodeDigits(tc.raw, tc.n, decodeOptions(skip)...)
				if got != want || !errors.Is(err, wantErr) {
					t.Fatalf("DecodeDigits(%x,%d,skip=%t)=%q,%v; want %q,%v", tc.raw, tc.n, skip, got, err, want, wantErr)
				}
			}
		})
	}
	// Framing errors ("short binary", overlength Address-Length and
	// alphanumeric rows) are checked by the caller, outside this codec.
}

// github.com/gomaja/go-sms, encoding/pdumode/smscaddress_test.go,
// TestSMSCAddressUnmarshalBinary; row names identify the source vectors.
// Only complete declared digit fields belong to Decode. Caller framing
// detects "nil", "short number", "short number pad", "underflow alpha",
// "underflow toa" and overlength fields before invoking the codec.
func TestSMSSMSCAddressVectors(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"only toa", nil, ""},
		{"only international toa", nil, ""},
		{"only fill", []byte{0xff}, ""},
		{"number", []byte{0x16, 0x04, 0x89, 0x56, 0x26, 0xf9}, "61409865629"},
		{"number alphabet", []byte{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe}, "0123456789*#abc"},
		{"alpha", []byte{0xed, 0xf2, 0x7c, 0x1e, 0x3e, 0x97, 0xe7}, "bc2a7c1c3797c"},
		{"toa without bit 7", []byte{0x16, 0x04, 0x89, 0x56, 0x26, 0xf9}, "61409865629"},
		{"zero length", nil, ""},
		{"max length", []byte{0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0x65, 0x87, 0x09}, "12345678901234567890"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(tc.raw, SkipInteriorFill())
			if got != tc.want || err != nil {
				t.Fatalf("Decode(%x,SkipInteriorFill())=%q,%v; want %q", tc.raw, got, err, tc.want)
			}
		})
	}
}
