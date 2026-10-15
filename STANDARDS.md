# Standards and scope

Versions checked on **2026-10-15** against the 3GPP specification archive, the
release trees under [Specs/latest](https://www.3gpp.org/ftp/Specs/latest/) and
the 3GPP portal records, which agree. The latest published version of each
document is the one listed.

| Document | Version | Clauses used | Archive | Portal |
| --- | --- | --- | --- | --- |
| 3GPP TS 29.002, *Mobile Application Part (MAP) specification* | V19.1.0 | §17.7.8 TBCD-STRING, AddressString; §17.7.1 GroupId, Long-GroupId, VoiceGroupCallData, VoiceBroadcastData | [29.002](https://www.3gpp.org/ftp/Specs/archive/29_series/29.002/) | [1585](https://portal.3gpp.org/desktopmodules/Specifications/SpecificationDetails.aspx?specificationId=1585) |
| 3GPP TS 23.040, *Technical realization of the Short Message Service (SMS)* | V20.0.0 | §9.1.2.3 Semi-octet representation; §9.1.2.5 Address fields | [23.040](https://www.3gpp.org/ftp/Specs/archive/23_series/23.040/) | [747](https://portal.3gpp.org/desktopmodules/Specifications/SpecificationDetails.aspx?specificationId=747) |
| 3GPP TS 24.008, *Mobile radio interface Layer 3 specification; Core network protocols; Stage 3* | V20.1.0 | §10.5.4.7 Called party BCD number, Table 10.5.118 | [24.008](https://www.3gpp.org/ftp/Specs/archive/24_series/24.008/) | [1015](https://portal.3gpp.org/desktopmodules/Specifications/SpecificationDetails.aspx?specificationId=1015) |

## Rules and where the code applies them

**Alphabet and digit order.** TS 29.002 §17.7.8 defines TBCD-STRING as "digits
from 0 through 9, *, #, a, b, c, two digits per octet, each digit encoded 0000
to 1001 (0 to 9), 1010 (*), 1011 (#), 1100 (a), 1101 (b) or 1110 (c)", with
"bits 8765 of octet n encoding digit 2n" and "bits 4321 of octet n encoding
digit 2(n-1) +1". TS 23.040 §9.1.2.3 puts the most significant digit in the
lowest octet and in "the half octet containing the bits with bit numbers 0 to
3", and has a receiver display 1010 to 1110 as `*`, `#`, `a`, `b` and `c`, the
values of the called party BCD number of TS 24.008, which "shall take
precedence" in a discrepancy. TS 24.008 Table 10.5.118 gives the same values.
`Encode`, `AppendEncode` and every decoder use this alphabet, and nothing else.

**Filler.** TS 29.002 has "1111 used as filler when there is an odd number of
digits"; TS 23.040 §9.1.2.3 says that then "the bits with bit numbers 4 to 7
within the last octet are fill bits and shall always be set to "1111""; TS
24.008 §10.5.4.7 NOTE 2 fills "bits 5 to 8 of the last octet" with "an end
mark coded as "1111"". `Encode` and `AppendEncode` write it.

**Strict decoding.** TS 29.002 has a filler only after the last of an odd
number of digits. `Decode`, `AppendDecode` and `DecodeDigits` reject a filler
that a digit follows, by default, rather than guess what the string means.

**Filler octets.** A GroupId (TS 29.002 §17.7.1, `TBCD-STRING (SIZE (3))`)
uses "the TBCD filler (1111) ... to fill unused half octets" when it has
fewer than six digits, and in VoiceGroupCallData and VoiceBroadcastData (also
§17.7.1) "shall be filled with six TBCD fillers (1111)" when a longGroupId is
present. `Decode` and `AppendDecode` therefore accept trailing filler octets
and drop them.

**Interior filler.** TS 23.040 §9.1.2.3: "If a mobile receives "1111" in a
position prior to the last semi-octet then processing shall commence with the
next semi-octet and the intervening semi-octet shall be ignored."
`SkipInteriorFill` selects this behaviour.

**Address-Length.** TS 23.040 §9.1.2.5: "The Address-Length field is an
integer representation of the number of useful semi-octets within the
Address-Value field, i.e. excludes any semi octet containing only fill bits."
`DecodeDigits(b, n)` decodes n digits from the first (n+1)/2 octets and does
not check the fill half-octet of an odd n, unless `SkipInteriorFill` skips a
filler among the digits and the fill half-octet then holds the last one.

## Out of scope

- Identity rules such as the digit counts of IMSI, IMEI and PLMN identities:
  `github.com/gomaja/go-3gpp-identity`.
- The swapped two-digit BCD of SMS time stamps and validity periods, and the
  type-of-address octets of address fields.
- The SCCP global title digits of ITU-T Q.713, which use a different code set.

Recheck the archive for newer versions before a significant revisit of this
package, and update this file with the date of the check.
