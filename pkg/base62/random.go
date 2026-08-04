package base62

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// RandomCode returns a random base62 string of the given length. Used for
// short codes instead of Encode(id) so codes aren't sequentially guessable.
func RandomCode(length int) (string, error) {
	b := make([]byte, length)
	max := big.NewInt(int64(base))

	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate random code: %w", err)
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}
