package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const length = 6

func GenerateGroupCode() (string, error) {
	code := make([]byte, length)

	// 字符表有 36 个字符，随机下标范围是 [0, 36)。
	limit := big.NewInt(int64(len(alphabet)))

	for i := range code {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate group code: %w", err)
		}

		code[i] = alphabet[index.Int64()]
	}

	return string(code), nil
}
