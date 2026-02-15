package platform

import (
	"crypto/rand"

	"github.com/google/uuid"
)

const shortIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const shortIDLength = 10

func NewID() string {
	return uuid.New().String()
}

func NewName(prefix string) string {
	b := make([]byte, shortIDLength)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	for i := range b {
		b[i] = shortIDAlphabet[b[i]%byte(len(shortIDAlphabet))]
	}
	return prefix + string(b)
}
