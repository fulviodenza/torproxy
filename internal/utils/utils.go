package utils

import (
	"crypto/rand"
	"math/big"
)

func GenerateName() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 4)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		result[i] = letters[num.Int64()]
	}
	return string(result)

}
