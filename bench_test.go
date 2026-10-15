package tbcd

import (
	"strings"
	"testing"
)

func BenchmarkCodec(b *testing.B) {
	for _, field := range []struct{ name, digits string }{
		{"IMSI15", "001010123456789"},
		{"AddressString40", strings.Repeat("1234567890", 4)},
		{"SMSAddress20", "12345678901234567890"},
	} {
		raw, err := Encode(field.digits)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(field.name, func(b *testing.B) {
			b.Run("Encode", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(field.digits)))
				for i := 0; i < b.N; i++ {
					resultBytes, resultError = Encode(field.digits)
				}
				if resultError != nil {
					b.Fatal(resultError)
				}
			})
			b.Run("AppendEncode", func(b *testing.B) {
				dst := make([]byte, 3, 3+len(raw))
				b.ReportAllocs()
				b.SetBytes(int64(len(field.digits)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					resultBytes, resultError = AppendEncode(dst, field.digits)
				}
				if resultError != nil {
					b.Fatal(resultError)
				}
			})
			for _, mode := range []struct {
				name string
				opts []DecodeOption
			}{
				{"strict", nil}, {"skip", []DecodeOption{SkipInteriorFill()}},
			} {
				b.Run("Decode/"+mode.name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(raw)))
					for i := 0; i < b.N; i++ {
						resultString, resultError = Decode(raw, mode.opts...)
					}
					if resultError != nil {
						b.Fatal(resultError)
					}
				})
				b.Run("AppendDecode/"+mode.name, func(b *testing.B) {
					dst := make([]byte, 3, 3+len(field.digits))
					b.ReportAllocs()
					b.SetBytes(int64(len(raw)))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						resultBytes, resultError = AppendDecode(dst, raw, mode.opts...)
					}
					if resultError != nil {
						b.Fatal(resultError)
					}
				})
				b.Run("DecodeDigits/"+mode.name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(raw)))
					for i := 0; i < b.N; i++ {
						resultString, resultError = DecodeDigits(raw, len(field.digits), mode.opts...)
					}
					if resultError != nil {
						b.Fatal(resultError)
					}
				})
			}
		})
	}
}
