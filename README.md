# go-tbcd

TBCD digit strings for Go: the TBCD-STRING of 3GPP TS 29.002 and the semi-octet
representation of 3GPP TS 23.040.

Both codings pack two digits per octet, the earlier digit in bits 4321 and the
later one in bits 8765, use the alphabet `0123456789*#abc`, and fill the high
half of the last octet with `1111` after an odd number of digits. MAP uses the
coding for IMSI, IMEI and the digits of an AddressString; SMS uses it for the
address fields of a TPDU.

## Status

The package holds the TBCD codec that `github.com/gomaja/go-asn1-gsmmap` used
internally, moved here unchanged. [Issue #1](https://github.com/gomaja/go-tbcd/issues/1)
describes the complete scope this module is to implement, so that
`go-asn1-gsmmap` and `github.com/gomaja/go-sms` can both use it instead of
their own copies.

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

`Encode` rejects characters outside the alphabet. `Decode` rejects a filler
that is followed by a digit and tolerates trailing filler octets.

## Specifications

[STANDARDS.md](STANDARDS.md) lists the documents and versions the code follows.

## License

MIT, see [LICENSE](LICENSE).
