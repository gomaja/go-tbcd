package tbcd

import (
	"bytes"
	"testing"
)

var codecSeeds = func() [][]byte {
	seeds := [][]byte{
		{}, {0xff}, {0xff, 0xff, 0xff}, {0x21, 0xf3}, {0x21, 0x4f}, {0x21, 0xff, 0x43}, {0x21, 0xf3, 0xff},
		{0x1a, 0x32, 0xfb}, {0xfe}, {0x00}, {0x99, 0xf9}, {0xaf},
		{0x10, 0x32, 0x54, 0x76}, {0x10, 0x32, 0x54, 0xf6}, {0x10, 0x32, 0xf4, 0x76, 0xf8}, {0x1f, 0x32, 0xff, 0x54},
		{0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe},
		// Remaining deterministic table fields, including unused trailing
		// octets, non-standard last half-octets and every filler position.
		{0x21, 0x43, 0x65, 0x87, 0x09},
		{0x21, 0x43, 0x15, 0x32, 0x54, 0x76, 0x98, 0xf0},
		{0xf5}, {0xfa}, {0xfb}, {0xfc}, {0xfd}, {0xba}, {0xdc, 0xfe},
		{0x1a, 0x32, 0xb4}, {0xf0}, {0x99}, {0x21, 0x43, 0xff},
		{0x21, 0xf3, 0xff, 0xff}, {0xf1, 0xff, 0xff}, {0x21, 0xff, 0xff},
		{0xff, 0x21}, {0x21, 0x3f, 0x65}, {0x0f}, {0xef}, {0xf1, 0x21}, {0xf1, 0xff, 0x05},
		{0x10, 0x32, 0x54, 0xf6, 0x98}, {0x10, 0x32, 0x54, 0x76, 0x98}, {0x10, 0x32, 0xf4, 0x76},
		{0xed, 0xf2, 0x7c, 0x1e, 0x3e, 0x97, 0xe7},
	}
	for i := 0; i < len(modelAlphabet); i++ {
		seeds = append(seeds, []byte{0xf0 | byte(i)}, []byte{byte(i)<<4 | 1})
	}
	return seeds
}()

func FuzzEncode(f *testing.F) {
	for _, s := range []string{"", "0", "12", "310150123456789", "*#", "*#0#", "**21*491711#", "#31#", "abcabc", modelAlphabet, "491711123456789012", "a", "00", "000", "012345D6789", "é", "1\xff", "*123#", "*1234#", "abc", "1234567890", "123451234567890", "09*#abc", "5", "99", "123g456", "A", "12B", "1C", "D", "E", "F", "12d", "e", "1234f", "+123", "12 34", "12-34", "1\x00", "12é", "12ı", "12š", "12Ī", "12\xc0\xaf"} {
		f.Add(s)
	}
	for _, c := range modelAlphabet {
		f.Add(string(c))
		f.Add("1" + string(c))
	}
	f.Fuzz(func(t *testing.T, s string) {
		want, wantErr := modelEncode(s)
		got, err := Encode(s)
		sameError(t, err, wantErr)
		if !bytes.Equal(got, want) || (err != nil && got != nil) || (err == nil && got == nil) {
			t.Fatalf("Encode(%q)=%x,%v; want %x,%v", s, got, err, want, wantErr)
		}
		backing := bytes.Repeat([]byte{0xa5}, 7+(len(s)+1)/2)
		copy(backing, "prefix:")
		before := bytes.Clone(backing)
		out, appendErr := AppendEncode(backing[:7], s)
		sameError(t, appendErr, wantErr)
		if err != nil {
			if len(out) != 7 || cap(out) != cap(backing) || &out[0] != &backing[0] || !bytes.Equal(backing, before) {
				t.Fatal("AppendEncode changed destination on error")
			}
			return
		}
		if string(out[:7]) != "prefix:" || !bytes.Equal(out[7:], got) {
			t.Fatalf("AppendEncode(%q)=%x; Encode=%x", s, out, got)
		}
		decoded, decodeErr := Decode(got)
		if decodeErr != nil || decoded != s {
			t.Fatalf("round trip %q=%q,%v", s, decoded, decodeErr)
		}
		checkDigitsModel(t, got, len(s), false)
		checkDigitsModel(t, got, len(s), true)
	})
}

func FuzzDecode(f *testing.F) {
	for _, raw := range codecSeeds {
		for _, skip := range []bool{false, true} {
			f.Add(raw, skip)
		}
	}
	for _, tc := range smsTPVectors {
		for _, skip := range []bool{false, true} {
			f.Add(tc.raw, skip)
		}
	}
	f.Fuzz(func(t *testing.T, raw []byte, skip bool) {
		checkDecodeModel(t, raw, skip)
		got, err := Decode(raw, decodeOptions(skip)...)
		if err != nil {
			return
		}
		encoded, err := Encode(got)
		if err != nil {
			t.Fatalf("decoded value cannot encode: %v", err)
		}
		if !skip && !bytes.Equal(encoded, trimFF(raw)) {
			t.Fatalf("canonical Decode(%x) encoded as %x", raw, encoded)
		}
		again, err := Decode(encoded, decodeOptions(skip)...)
		if err != nil || again != got {
			t.Fatalf("canonical round trip %q=%q,%v", got, again, err)
		}
	})
}

func FuzzDecodeDigits(f *testing.F) {
	for _, raw := range codecSeeds {
		for _, n := range []int{-1, 0, 1, 2, 3, 4, 5, 2*len(raw) - 1, 2 * len(raw), 2*len(raw) + 1} {
			for _, skip := range []bool{false, true} {
				f.Add(raw, n, skip)
			}
		}
	}
	for _, tc := range smsTPVectors {
		for _, skip := range []bool{false, true} {
			f.Add(tc.raw, tc.n, skip)
		}
	}
	f.Fuzz(func(t *testing.T, raw []byte, n int, skip bool) {
		// Keep small positive seed counts unchanged, exercise short and
		// sufficient input, and retain negative counts without negating MinInt.
		n %= 2*len(raw) + 3
		checkDigitsModel(t, raw, n, skip)
		got, err := DecodeDigits(raw, n, decodeOptions(skip)...)
		if err != nil {
			return
		}
		encoded, err := Encode(got)
		if err != nil {
			t.Fatalf("decoded value cannot encode: %v", err)
		}
		if !skip {
			canonical := bytes.Clone(raw[:n/2+n%2])
			if n%2 != 0 {
				canonical[len(canonical)-1] |= 0xf0
			}
			if !bytes.Equal(encoded, canonical) {
				t.Fatalf("DecodeDigits(%x,%d) encoded as %x; want %x", raw, n, encoded, canonical)
			}
		}
		again, err := DecodeDigits(encoded, n, decodeOptions(skip)...)
		if err != nil || again != got {
			t.Fatalf("canonical digit round trip %q=%q,%v", got, again, err)
		}
	})
}
