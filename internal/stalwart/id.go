package stalwart

// crockfordAlphabet is the lowercase Crockford base32 alphabet.
const crockfordAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// EncodePrincipalID encodes a Stalwart principal ID as a lowercase Crockford
// base32 string, matching the internal account ID format used by Stalwart's
// JMAP API.
func EncodePrincipalID(id uint32) string {
	if id == 0 {
		return "0"
	}

	// Maximum 7 digits for uint32 in base32.
	var buf [7]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = crockfordAlphabet[id%32]
		id /= 32
	}
	return string(buf[i:])
}
