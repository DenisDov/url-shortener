// Package base62 encodes integers into short, URL-safe strings and back.
// Used as one strategy for generating short codes from an auto-increment ID.
package base62

import "strings"

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const base = uint64(len(alphabet))

// Encode converts a positive integer into a base62 string.
// 0 -> "0".
func Encode(n uint64) string {
	if n == 0 {
		return string(alphabet[0])
	}

	var sb strings.Builder
	for n > 0 {
		sb.WriteByte(alphabet[n%base])
		n /= base
	}

	// digits were written least-significant first, reverse them
	encoded := []byte(sb.String())
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded)
}

// Decode converts a base62 string back into its integer value.
func Decode(s string) uint64 {
	var n uint64
	for _, r := range s {
		n = n*base + uint64(strings.IndexRune(alphabet, r))
	}
	return n
}
