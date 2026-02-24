package stalwart

// stalwartAlphabet is the custom base32 alphabet used by Stalwart internally
// for encoding principal IDs into JMAP account IDs. This is NOT Crockford
// base32 — Stalwart uses a-z for values 0-25, then 7,9,2,0,1,3 for 26-31.
// See: https://github.com/stalwartlabs/stalwart/blob/main/crates/utils/src/codec/base32_custom.rs
const stalwartAlphabet = "abcdefghijklmnopqrstuvwxyz792013"

// EncodePrincipalID encodes a Stalwart principal ID using Stalwart's custom
// base32 alphabet, matching the internal account ID format used by Stalwart's
// JMAP API.
func EncodePrincipalID(id uint32) string {
	if id == 0 {
		return "a"
	}

	// Maximum 7 digits for uint32 in base32.
	var buf [7]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = stalwartAlphabet[id%32]
		id /= 32
	}
	return string(buf[i:])
}
