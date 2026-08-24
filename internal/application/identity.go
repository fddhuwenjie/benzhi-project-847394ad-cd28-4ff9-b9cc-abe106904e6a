package application

import (
	"crypto/rand"
	"encoding/hex"
)

func newID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("系统随机源不可用: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(b)
}
