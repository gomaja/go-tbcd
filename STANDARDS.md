# Standards and scope

Versions checked against the 3GPP specification archive on **2026-10-15**. The
latest published version of each document is the one listed.

| Document | Version | Clauses used | Archive |
| --- | --- | --- | --- |
| 3GPP TS 29.002, *Mobile Application Part (MAP) specification* | V19.1.0 | §17.7.8 TBCD-STRING | [29.002](https://www.3gpp.org/ftp/Specs/archive/29_series/29.002/) |
| 3GPP TS 23.040, *Technical realization of the Short Message Service (SMS)* | V20.0.0 | §9.1.2.3 Semi-octet representation | [23.040](https://www.3gpp.org/ftp/Specs/archive/23_series/23.040/) |

TS 29.002 defines the TBCD-STRING alphabet as the digits 0 to 9, `*`, `#`, `a`,
`b` and `c`, coded 0000 to 1110, with 1111 as the filler for an odd number of
digits. TS 23.040 codes decimal digits the same way and tells a receiver to show
the semi-octets 1010 to 1110 as `*`, `#`, `a`, `b` and `c`.

Recheck the archive for newer versions before a significant revisit of this
package, and update this file with the date of the check.
