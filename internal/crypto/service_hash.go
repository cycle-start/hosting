package crypto

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// MysqlNativePasswordHash computes the mysql_native_password hash:
// "*" + HEX(SHA1(SHA1(password))). This hash can be passed directly to
// MySQL via IDENTIFIED ... AS '*hash' syntax.
func MysqlNativePasswordHash(password string) string {
	first := sha1.Sum([]byte(password))
	second := sha1.Sum(first[:])
	return "*" + strings.ToUpper(hex.EncodeToString(second[:]))
}

// ValkeyPasswordHash computes the SHA-256 hex hash used by Valkey ACL.
// The hash can be passed to Valkey via the #hash syntax in ACL commands.
func ValkeyPasswordHash(password string) string {
	h := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", h)
}

// GenericHash computes a SHA-256 hex hash for audit storage.
func GenericHash(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return fmt.Sprintf("%x", h)
}
