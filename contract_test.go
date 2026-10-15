package tbcd

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

// TS 29.002 V19.1.0 §17.7.8 and TS 24.008 V20.1.0 §10.5.4.7,
// Table 10.5.118 define this mapping. The model never uses codec helpers.
const modelAlphabet = "0123456789*#abc"

func modelEncode(s string) ([]byte, error) {
	for i, r := range s {
		if !strings.ContainsRune(modelAlphabet, r) {
			return nil, fmt.Errorf("%w: %q at index %d", ErrInvalidCharacter, r, i)
		}
	}
	digits := s
	if len(digits)%2 != 0 {
		digits += "F"
	}
	out := make([]byte, 0, len(digits)/2)
	for i := 0; i < len(digits); i += 2 {
		lo := strings.IndexByte(modelAlphabet+"F", digits[i])
		hi := strings.IndexByte(modelAlphabet+"F", digits[i+1])
		out = append(out, byte(lo+16*hi))
	}
	return out, nil
}

// Expand first, then apply filler and count rules as slice operations. SMS
// tolerances follow TS 23.040 V20.0.0 §§9.1.2.3 and 9.1.2.5.
func modelExpand(src []byte) []byte {
	digits := make([]byte, 0, 2*len(src))
	for _, o := range src {
		digits = append(digits, (modelAlphabet + "F")[int(o)%16], (modelAlphabet + "F")[int(o)/16])
	}
	return digits
}

func modelDecode(src []byte, skip bool) (string, error) {
	digits := modelExpand(src)
	first := bytes.IndexByte(digits, 'F')
	if !skip && first >= 0 {
		for i := first + 1; i < len(digits); i++ {
			if digits[i] != 'F' {
				return "", fmt.Errorf("%w: octet %d (0x%02x) has a digit after a filler", ErrMisplacedFiller, i/2, src[i/2])
			}
		}
	}
	return strings.ReplaceAll(string(digits), "F", ""), nil
}

func modelDecodeDigits(src []byte, n int, skip bool) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("%w: %d", ErrNegativeCount, n)
	}
	window := n/2 + n%2
	if len(src) < window {
		return "", fmt.Errorf("%w: %d digits need %d octets, have %d", ErrShortInput, n, window, len(src))
	}
	digits := modelExpand(src[:window])
	if skip {
		useful := strings.ReplaceAll(string(digits), "F", "")
		if len(useful) < n {
			return "", fmt.Errorf("%w: %d octets hold %d of %d digits after skipping fillers", ErrShortInput, window, len(useful), n)
		}
		return useful[:n], nil
	}
	required := digits[:n]
	if i := bytes.IndexByte(required, 'F'); i >= 0 {
		return "", fmt.Errorf("%w: octet %d (0x%02x) has a filler as digit %d of %d", ErrMisplacedFiller, i/2, src[i/2], i+1, n)
	}
	return string(required), nil
}

func decodeOptions(skip bool) []DecodeOption {
	if skip {
		return []DecodeOption{SkipInteriorFill()}
	}
	return nil
}

func sameError(t testing.TB, got, want error) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("error = %v; want %v", got, want)
	}
	if want == nil {
		return
	}
	for _, sentinel := range []error{ErrInvalidCharacter, ErrMisplacedFiller, ErrShortInput, ErrNegativeCount} {
		if errors.Is(got, sentinel) != errors.Is(want, sentinel) {
			t.Fatalf("error %v: errors.Is(%v) disagrees with %v", got, sentinel, want)
		}
	}
	if got.Error() != want.Error() {
		t.Fatalf("error = %q; want %q", got, want)
	}
}

func checkDecodeModel(t testing.TB, raw []byte, skip bool) {
	t.Helper()
	opts := decodeOptions(skip)
	want, wantErr := modelDecode(raw, skip)
	got, err := Decode(raw, opts...)
	sameError(t, err, wantErr)
	if got != want {
		t.Fatalf("Decode(%x, skip=%t) = %q; want %q", raw, skip, got, want)
	}
	backing := bytes.Repeat([]byte{0xa5}, 7+2*len(raw))
	copy(backing, "prefix:")
	before := bytes.Clone(backing)
	dst := backing[:7]
	out, err := AppendDecode(dst, raw, opts...)
	sameError(t, err, wantErr)
	if wantErr != nil {
		if len(out) != len(dst) || cap(out) != cap(dst) || &out[0] != &dst[0] || !bytes.Equal(backing, before) {
			t.Fatalf("AppendDecode(%x) changed dst on error: out=%x, backing=%x", raw, out, backing)
		}
	} else if string(out) != "prefix:"+want {
		t.Fatalf("AppendDecode(%x, skip=%t) = %q; want prefix + %q", raw, skip, out, want)
	} else if !bytes.Equal(backing[len(out):], before[len(out):]) {
		t.Fatalf("AppendDecode(%x) wrote beyond output length", raw)
	}
}

func checkDigitsModel(t testing.TB, raw []byte, n int, skip bool) {
	t.Helper()
	want, wantErr := modelDecodeDigits(raw, n, skip)
	got, err := DecodeDigits(raw, n, decodeOptions(skip)...)
	sameError(t, err, wantErr)
	if got != want {
		t.Fatalf("DecodeDigits(%x,%d, skip=%t) = %q; want %q", raw, n, skip, got, want)
	}
}

func TestReferenceModelExhaustive(t *testing.T) {
	total := 0
	check := func(raw []byte) {
		for _, skip := range []bool{false, true} {
			checkDecodeModel(t, raw, skip)
			for n := 0; n <= 5; n++ {
				checkDigitsModel(t, raw, n, skip)
			}
		}
		total++
	}
	check(nil)
	for a := 0; a < 256; a++ {
		check([]byte{byte(a)})
		for b := 0; b < 256; b++ {
			check([]byte{byte(a), byte(b)})
		}
	}
	t.Logf("compared %d byte strings in both modes and digit counts 0..5", total)
}

func checkValidString(t testing.TB, s string) {
	t.Helper()
	want, err := modelEncode(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Encode(s)
	if err != nil || got == nil || !bytes.Equal(got, want) {
		t.Fatalf("Encode(%q) = %x,%v; want %x", s, got, err, want)
	}
	backing := bytes.Repeat([]byte{0xa5}, 7+len(want))
	copy(backing, "prefix:")
	appended, err := AppendEncode(backing[:7], s)
	if err != nil || !bytes.Equal(appended[7:], want) || string(appended[:7]) != "prefix:" {
		t.Fatalf("AppendEncode(%q) = %x,%v", s, appended, err)
	}
	for _, skip := range []bool{false, true} {
		checkDecodeModel(t, got, skip)
		decoded, err := Decode(got, decodeOptions(skip)...)
		if err != nil || decoded != s {
			t.Fatalf("round trip %q = %q,%v", s, decoded, err)
		}
		digits, err := DecodeDigits(got, len(s), decodeOptions(skip)...)
		if err != nil || digits != s {
			t.Fatalf("digit round trip %q = %q,%v", s, digits, err)
		}
	}
}

func TestValidStringsExhaustive(t *testing.T) {
	total := 0
	var visit func(string, int)
	visit = func(s string, remaining int) {
		checkValidString(t, s)
		total++
		if remaining == 0 {
			return
		}
		for _, c := range modelAlphabet {
			visit(s+string(c), remaining-1)
		}
	}
	visit("", 4)
	t.Logf("round-tripped %d strings over the complete alphabet", total)
}

func TestValidStringsRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x74626364, 0x12345678))
	for i := 0; i < 2000; i++ {
		raw := make([]byte, rng.IntN(49))
		for j := range raw {
			raw[j] = modelAlphabet[rng.IntN(len(modelAlphabet))]
		}
		checkValidString(t, string(raw))
	}
}

func TestIssueTable(t *testing.T) {
	for _, tc := range []struct {
		raw                       []byte
		n                         int
		whole, digits             string
		strictWhole, strictDigits error
	}{
		{[]byte{0x21, 0xf3}, 3, "123", "123", nil, nil},
		{[]byte{0x21, 0x4f}, 3, "124", "124", ErrMisplacedFiller, ErrMisplacedFiller},
		{[]byte{0x21, 0xff, 0x43}, 4, "1234", "", ErrMisplacedFiller, ErrMisplacedFiller},
		{[]byte{0x21, 0xf3, 0xff}, 3, "123", "123", nil, nil},
	} {
		t.Run(fmt.Sprintf("%x", tc.raw), func(t *testing.T) {
			for _, skip := range []bool{false, true} {
				got, err := Decode(tc.raw, decodeOptions(skip)...)
				wantErr := tc.strictWhole
				want := tc.whole
				if skip {
					wantErr = nil
				}
				if wantErr != nil {
					want = ""
				}
				if !errors.Is(err, wantErr) || got != want {
					t.Fatalf("Decode(skip=%t)=%q,%v; want %q,%v", skip, got, err, want, wantErr)
				}
				wantErr = tc.strictDigits
				want = tc.digits
				if skip {
					wantErr = nil
					if want == "" {
						wantErr = ErrShortInput
					}
				}
				if wantErr != nil {
					want = ""
				}
				got, err = DecodeDigits(tc.raw, tc.n, decodeOptions(skip)...)
				if !errors.Is(err, wantErr) || got != want {
					t.Fatalf("DecodeDigits(skip=%t)=%q,%v; want %q,%v", skip, got, err, want, wantErr)
				}
			}
		})
	}
}

func TestEncodeErrorsAndAtomicAppend(t *testing.T) {
	for _, s := range []string{"A", "12g", "12é", "12ı", "12š", "12Ī", "123😀", "1\xff", "12\xc0\xaf", "1\x00", "123\n", "1234d"} {
		t.Run(fmt.Sprintf("%x", s), func(t *testing.T) {
			_, wantErr := modelEncode(s)
			out, err := Encode(s)
			sameError(t, err, wantErr)
			if out != nil {
				t.Fatalf("Encode(%q) returned %x on error", s, out)
			}
			// Cover nil, zero-length non-nil and non-empty destinations, with
			// both enough capacity and a growth requirement before the error.
			for _, length := range []int{0, 3} {
				for _, spare := range []int{0, 32} {
					backing := bytes.Repeat([]byte{0xa5}, length+spare)
					before := bytes.Clone(backing)
					dst := backing[:length]
					got, err := AppendEncode(dst, s)
					sameError(t, err, wantErr)
					if len(got) != len(dst) || cap(got) != cap(dst) || !bytes.Equal(backing, before) {
						t.Fatalf("AppendEncode(%q) changed dst", s)
					}
					if cap(dst) > 0 && &got[:cap(got)][0] != &dst[:cap(dst)][0] {
						t.Fatal("AppendEncode replaced backing array on error")
					}
				}
			}
			got, err := AppendEncode(nil, s)
			sameError(t, err, wantErr)
			if got != nil {
				t.Fatalf("AppendEncode(nil,%q)=%x", s, got)
			}
		})
	}
}

func TestDecodeErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		raw  []byte
		want string
	}{
		{[]byte{0x1f}, "tbcd: misplaced filler nibble: octet 0 (0x1f) has a digit after a filler"},
		{[]byte{0xf1, 0xff, 0x32}, "tbcd: misplaced filler nibble: octet 2 (0x32) has a digit after a filler"},
	} {
		got, err := Decode(tc.raw)
		if got != "" || !errors.Is(err, ErrMisplacedFiller) || err.Error() != tc.want {
			t.Fatalf("Decode(%x)=%q,%v; want %s", tc.raw, got, err, tc.want)
		}
		for _, dst := range [][]byte{nil, {}, make([]byte, 0, 32), make([]byte, 3, 32), make([]byte, 3)} {
			for i := range dst[:cap(dst)] {
				dst[:cap(dst)][i] = 0xa5
			}
			before := bytes.Clone(dst[:cap(dst)])
			out, err := AppendDecode(dst, tc.raw)
			if !errors.Is(err, ErrMisplacedFiller) || err.Error() != tc.want || len(out) != len(dst) || cap(out) != cap(dst) || (out == nil) != (dst == nil) || !bytes.Equal(dst[:cap(dst)], before) {
				t.Fatalf("AppendDecode(%x) changed destination or error", tc.raw)
			}
			if cap(dst) > 0 && &dst[:cap(dst)][0] != &out[:cap(out)][0] {
				t.Fatal("AppendDecode replaced backing array on error")
			}
		}
	}
}

func TestDecodeDigitsEdges(t *testing.T) {
	for _, tc := range []struct {
		raw      []byte
		n        int
		skip     bool
		want     string
		sentinel error
		message  string
	}{
		{nil, -1, false, "", ErrNegativeCount, "tbcd: negative digit count: -1"},
		{[]byte{0x21}, 3, false, "", ErrShortInput, "tbcd: input too short for the digit count: 3 digits need 2 octets, have 1"},
		{[]byte{0x21, 0xf3}, 4, false, "", ErrMisplacedFiller, "tbcd: misplaced filler nibble: octet 1 (0xf3) has a filler as digit 4 of 4"},
		{[]byte{0x21, 0x3f}, 3, false, "", ErrMisplacedFiller, "tbcd: misplaced filler nibble: octet 1 (0x3f) has a filler as digit 3 of 3"},
		{[]byte{0xf1, 0xf3}, 3, true, "", ErrShortInput, "tbcd: input too short for the digit count: 2 octets hold 2 of 3 digits after skipping fillers"},
		{[]byte{0xf1, 0xf3, 0x54}, 3, true, "", ErrShortInput, "tbcd: input too short for the digit count: 2 octets hold 2 of 3 digits after skipping fillers"},
		{[]byte{0xff, 0x21}, 1, true, "", ErrShortInput, "tbcd: input too short for the digit count: 1 octets hold 0 of 1 digits after skipping fillers"},
		{nil, 0, false, "", nil, ""},
		{[]byte{0x1f}, 0, false, "", nil, ""},
		{[]byte{0xff}, 0, true, "", nil, ""},
		{[]byte{0xf1, 0x32, 0xef}, 3, true, "123", nil, ""},
	} {
		got, err := DecodeDigits(tc.raw, tc.n, decodeOptions(tc.skip)...)
		if got != tc.want || !errors.Is(err, tc.sentinel) {
			t.Fatalf("DecodeDigits(%x,%d,skip=%t)=%q,%v", tc.raw, tc.n, tc.skip, got, err)
		}
		if err != nil && err.Error() != tc.message {
			t.Fatalf("error = %q, want %q", err, tc.message)
		}
	}
	for _, n := range []int{math.MinInt, math.MaxInt, math.MaxInt - 1} {
		for _, skip := range []bool{false, true} {
			checkDigitsModel(t, []byte{0x21}, n, skip)
			allocs := testing.AllocsPerRun(10, func() { _, _ = DecodeDigits(nil, n, decodeOptions(skip)...) })
			if allocs > 12 {
				t.Fatalf("DecodeDigits(nil,%d) allocated %.0f objects; want bounded error allocation", n, allocs)
			}
		}
	}
	// Every possible unused high half-octet must be ignored; following
	// octets would either add digits or cause strict filler errors if read.
	for hi := 0; hi < 16; hi++ {
		raw := []byte{0x21, byte(hi*16 + 3), 0xff, 0x1f, 0x98}
		for _, skip := range []bool{false, true} {
			got, err := DecodeDigits(raw, 3, decodeOptions(skip)...)
			if got != "123" || err != nil {
				t.Fatalf("unused nibble %x: %q,%v", hi, got, err)
			}
			got, err = DecodeDigits(raw, 2, decodeOptions(skip)...)
			if got != "12" || err != nil {
				t.Fatalf("unused octets: %q,%v", got, err)
			}
		}
	}
}

func TestDecodeOptionsAndAppendGrowth(t *testing.T) {
	for _, opts := range [][]DecodeOption{nil, {{}}, {{}, {}}} {
		got, err := Decode([]byte{0x21, 0xf3}, opts...)
		if got != "123" || err != nil {
			t.Fatalf("zero option changed Decode: %q,%v", got, err)
		}
		got, err = DecodeDigits([]byte{0x21, 0xf3}, 3, opts...)
		if got != "123" || err != nil {
			t.Fatalf("zero option changed DecodeDigits: %q,%v", got, err)
		}
		_, err = Decode([]byte{0xf1, 0x32}, opts...)
		if !errors.Is(err, ErrMisplacedFiller) {
			t.Fatalf("zero option suppressed error: %v", err)
		}
		_, err = DecodeDigits([]byte{0xf1, 0x32}, 3, opts...)
		if !errors.Is(err, ErrMisplacedFiller) {
			t.Fatalf("zero option suppressed digit error: %v", err)
		}
	}
	for _, opts := range [][]DecodeOption{{{}, SkipInteriorFill()}, {SkipInteriorFill(), {}}, {SkipInteriorFill(), SkipInteriorFill()}} {
		got, err := Decode([]byte{0xf1, 0x32}, opts...)
		if got != "123" || err != nil {
			t.Fatalf("option composition: %q,%v", got, err)
		}
		got, err = DecodeDigits([]byte{0xf1, 0x32}, 3, opts...)
		if got != "123" || err != nil {
			t.Fatalf("digit option composition: %q,%v", got, err)
		}
	}
	for _, initial := range [][]byte{nil, {}, []byte("prefix:")} {
		out, err := AppendEncode(initial, "123")
		if err != nil || !bytes.Equal(out, append(bytes.Clone(initial), 0x21, 0xf3)) {
			t.Fatalf("AppendEncode growth: %x,%v", out, err)
		}
		for _, skip := range []bool{false, true} {
			decoded, err := AppendDecode(initial, []byte{0x21, 0xf3}, decodeOptions(skip)...)
			if err != nil || string(decoded) != string(initial)+"123" {
				t.Fatalf("AppendDecode growth: %q,%v", decoded, err)
			}
		}
	}
}
