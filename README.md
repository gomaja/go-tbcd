# go-tbcd

TBCD digit strings for Go: the TBCD-STRING of 3GPP TS 29.002 and the semi-octet
representation of 3GPP TS 23.040.

Both codings pack two digits per octet, the earlier digit in bits 4321 and the
later one in bits 8765, use the alphabet `0123456789*#abc`, and fill the high
half of the last octet with `1111` after an odd number of digits. MAP uses the
coding for IMSI, IMEI and the digits of an AddressString; SMS uses it for the
address fields of a TPDU and for the SC address.

The package depends on the standard library only and needs Go 1.23 or later.

## Install

The module is used from its main branch; there are no tagged releases.

```
go get github.com/gomaja/go-tbcd@main
```

## Usage

```go
b, err := tbcd.Encode("123451234567890") // 21 43 15 32 54 76 98 f0
s, err := tbcd.Decode(b)                 // "123451234567890"
```

`Encode` rejects characters outside the alphabet. The two specifications treat
a filler that a digit follows differently, and the decoders offer both:

| Input | `Decode(b)` | `Decode(b, tbcd.SkipInteriorFill())` |
| --- | --- | --- |
| `21 f3` | `"123"` | `"123"` |
| `21 4f` (filler, then a digit) | `ErrMisplacedFiller` | `"124"` |
| `21 ff 43` (filler octet in the middle) | `ErrMisplacedFiller` | `"1234"` |
| `21 f3 ff` (trailing filler octet) | `"123"` | `"123"` |

- By default the decoders are strict, as MAP needs: a filler may only follow
  the last digit. `Decode` accepts trailing filler octets, which fixed-size
  fields such as GroupId carry, and drops them.
- `SkipInteriorFill` skips a filler anywhere, as TS 23.040 has an SMS receiver
  do.
- `DecodeDigits(b, n)` decodes n digits from the first (n+1)/2 octets, as an
  SMS address field is decoded from its Address-Length. It does not check the
  fill half-octet of an odd n. With `SkipInteriorFill`, a filler among the
  digits is skipped and the half-octets after it move up, so that the fill
  half-octet can then hold the last digit.

```go
// TP-DA: Address-Length 11, type of address 0x91, then the digits.
field := []byte{0x0b, 0x91, 0x16, 0x04, 0x89, 0x56, 0x26, 0xf9}
s, err := tbcd.DecodeDigits(field[2:], int(field[0]), tbcd.SkipInteriorFill()) // "61409865629"
```

| Field | Call |
| --- | --- |
| MAP TBCD-STRING, digits of an AddressString | `Decode(b)` |
| TS 23.040 TP-OA, TP-DA, TP-RA with a semi-octet address (an alphanumeric one, type of number 101, is in the GSM 7 bit alphabet instead) | `DecodeDigits(b, addressLength, SkipInteriorFill())` |
| SC address in front of a PDU-mode TPDU | `Decode(b, SkipInteriorFill())` |

`AppendEncode` and `AppendDecode` append to a caller's buffer and allocate only
to grow it. Errors wrap `ErrInvalidCharacter`, `ErrMisplacedFiller`,
`ErrShortInput` or `ErrNegativeCount` and name the position; match them with
`errors.Is`.

## Specifications

[STANDARDS.md](STANDARDS.md) lists the documents and versions the code follows,
and which rule each part of the API applies.

## License

MIT, see [LICENSE](LICENSE).
