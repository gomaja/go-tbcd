package tbcd_test

import (
	"fmt"

	"github.com/gomaja/go-tbcd"
)

func Example() {
	// A synthetic 15-digit IMSI uses eight octets, with a final filler.
	encoded, err := tbcd.Encode("001010123456789")
	if err != nil {
		panic(err)
	}
	fmt.Printf("% x\n", encoded)
	decoded, err := tbcd.Decode(encoded)
	if err != nil {
		panic(err)
	}
	fmt.Println(decoded)
	// Output:
	// 00 01 01 21 43 65 87 f9
	// 001010123456789
}

func ExampleEncode() {
	encoded, err := tbcd.Encode("*123#")
	fmt.Printf("% x; %v\n", encoded, err)
	// Output: 1a 32 fb; <nil>
}

func ExampleDecode() {
	digits, err := tbcd.Decode([]byte{0x21, 0xf3, 0xff})
	fmt.Printf("%s; %v\n", digits, err)
	// Output: 123; <nil>
}

func ExampleDecode_skipInteriorFill() {
	// An SC address has an octet count; decode its complete digit field.
	// TS 23.040 V20.0.0 §9.1.2.3 has receivers skip interior fill.
	digits, err := tbcd.Decode([]byte{0x21, 0x4f, 0x65}, tbcd.SkipInteriorFill())
	fmt.Printf("%s; %v\n", digits, err)
	// Output: 12456; <nil>
}

func ExampleDecodeDigits() {
	// TP-DA: Address-Length 3, Type-of-Address 0x81, two digit octets.
	// TS 23.040 V20.0.0 §9.1.2.5 excludes the unused last half-octet:
	// its received value 0xa carries no digit and is ignored.
	tpDA := []byte{3, 0x81, 0x21, 0xa3}
	digits, err := tbcd.DecodeDigits(tpDA[2:], int(tpDA[0]), tbcd.SkipInteriorFill())
	fmt.Printf("%s; %v\n", digits, err)
	// Output: 123; <nil>
}

func ExampleAppendEncode() {
	field, err := tbcd.AppendEncode([]byte{3, 0x81}, "123")
	fmt.Printf("% x; %v\n", field, err)
	// Output: 03 81 21 f3; <nil>
}

func ExampleAppendDecode() {
	label, err := tbcd.AppendDecode([]byte("digits: "), []byte{0x21, 0xf3})
	fmt.Printf("%s; %v\n", label, err)
	// Output: digits: 123; <nil>
}
