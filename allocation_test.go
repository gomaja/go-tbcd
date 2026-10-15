package tbcd

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

var (
	resultBytes  []byte
	resultString string
	resultError  error
)

// allocCase is a call whose allocations are counted, with the most it may
// make.
type allocCase struct {
	name string
	max  float64
	call func()
}

func TestAllocations(t *testing.T) {
	for octets := 0; octets <= 32; octets++ {
		for _, odd := range []bool{false, true} {
			digits := 2 * octets
			if odd && digits > 0 {
				digits--
			}
			s := strings.Repeat("1", digits)
			raw, err := modelEncode(s)
			if err != nil {
				t.Fatal(err)
			}
			dst := make([]byte, 7, 7+len(s))
			copy(dst, "prefix:")
			cases := []allocCase{
				{"Encode", 1, func() { resultBytes, resultError = Encode(s) }},
				{"AppendEncode", 0, func() { resultBytes, resultError = AppendEncode(dst, s) }},
			}
			for _, mode := range []struct {
				name string
				opts []DecodeOption
			}{
				{"strict", nil}, {"skip", []DecodeOption{SkipInteriorFill()}}, {"zero", []DecodeOption{{}}},
			} {
				opts := mode.opts
				cases = append(cases,
					allocCase{"Decode/" + mode.name, 1, func() { resultString, resultError = Decode(raw, opts...) }},
					allocCase{"AppendDecode/" + mode.name, 0, func() { resultBytes, resultError = AppendDecode(dst, raw, opts...) }},
					allocCase{"DecodeDigits/" + mode.name, 1, func() { resultString, resultError = DecodeDigits(raw, digits, opts...) }},
				)
			}
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/%d/odd=%t", tc.name, octets, odd), func(t *testing.T) {
					allocs := testing.AllocsPerRun(50, tc.call)
					if resultError != nil {
						t.Fatal(resultError)
					}
					if allocs > tc.max {
						t.Fatalf("%.0f allocations, want <= %.0f", allocs, tc.max)
					}
				})
			}
		}
	}
	// Exercise the skip path with an actual interior filler, as well as an
	// odd number of useful digits that extends into the final half-octet.
	dst := make([]byte, 3, 64)
	for _, raw := range [][]byte{{0xf1, 0x32}, {0xff, 0x21, 0xf3, 0x54}} {
		allocs := testing.AllocsPerRun(100, func() { resultBytes, resultError = AppendDecode(dst, raw, SkipInteriorFill()) })
		if resultError != nil || allocs != 0 {
			t.Fatalf("AppendDecode(%x,skip): %.0f allocations,%v", raw, allocs, resultError)
		}
	}
}

// Encode checks the whole string before it allocates the coding, so a long
// string with an invalid last character costs only the error.
func TestEncodeChecksBeforeAllocating(t *testing.T) {
	s := strings.Repeat("1", 1<<16) + "x"
	resultBytes, resultError = Encode(s)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 4; i++ {
		resultBytes, resultError = Encode(s)
	}
	runtime.ReadMemStats(&after)
	if resultBytes != nil || !errors.Is(resultError, ErrInvalidCharacter) {
		t.Fatalf("Encode = %x, %v; want nil, ErrInvalidCharacter", resultBytes, resultError)
	}
	if perCall := (after.TotalAlloc - before.TotalAlloc) / 4; perCall > 1024 {
		t.Fatalf("Encode of an invalid %d-character string allocated %d B/call", len(s), perCall)
	}
}

// Total allocation bytes measure space growth without a timing threshold.
// Each doubling must remain within a fixed per-input budget. This catches
// repeated concatenation/allocation but does not claim a CPU-time bound.
func TestLinearAllocationGrowth(t *testing.T) {
	for size := 1024; size <= 1024*1024; size *= 2 {
		raw := bytes.Repeat([]byte{0x21}, size)
		digits := strings.Repeat("12", size)
		dst := make([]byte, 3, 3+2*size)
		// max is the budget in allocated bytes per input octet.
		for _, tc := range []allocCase{
			{"Encode", 2, func() { resultBytes, resultError = Encode(digits) }},
			{"AppendEncode", 0, func() { resultBytes, resultError = AppendEncode(dst, digits) }},
			{"Decode", 8, func() { resultString, resultError = Decode(raw) }},
			{"DecodeSkip", 8, func() { resultString, resultError = Decode(raw, SkipInteriorFill()) }},
			{"AppendDecode", 0, func() { resultBytes, resultError = AppendDecode(dst, raw) }},
			{"AppendDecodeSkip", 0, func() { resultBytes, resultError = AppendDecode(dst, raw, SkipInteriorFill()) }},
			{"DecodeDigits", 5, func() { resultString, resultError = DecodeDigits(raw, 2*len(raw)) }},
			{"DecodeDigitsSkip", 5, func() { resultString, resultError = DecodeDigits(raw, 2*len(raw), SkipInteriorFill()) }},
		} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, size), func(t *testing.T) {
				// Warm lazy runtime state before the measured interval. A small
				// constant allowance covers unrelated runtime bookkeeping.
				tc.call()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				for i := 0; i < 4; i++ {
					tc.call()
				}
				runtime.ReadMemStats(&after)
				perCall := (after.TotalAlloc - before.TotalAlloc) / 4
				if resultError != nil {
					t.Fatal(resultError)
				}
				budget := uint64(tc.max)*uint64(size) + 4096
				if perCall > budget {
					t.Fatalf("%d B/call for %d input octets, budget %d", perCall, size, budget)
				}
				t.Logf("%d input octets: %d B/call", size, perCall)
			})
		}
	}
}
