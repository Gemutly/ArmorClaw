package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
)

var (
	ErrInvalidPhraseLength = errors.New("recovery phrase must be exactly 24 words")
	ErrInvalidChecksum     = errors.New("recovery phrase checksum verification failed")
	ErrWordNotInList       = errors.New("word not found in BIP-39 wordlist")
)

// bip39EnglishWordlist is the standard BIP-39 English wordlist (2048 words).
//go:embed wordlist.txt
var bip39EnglishWordlist []byte

// bip39Words holds the parsed wordlist. Initialized lazily.
var bip39Words []string

// bip39WordMap maps word → index for fast lookup. Initialized lazily.
var bip39WordMap map[string]int

// ensureWordlistLoaded parses the embedded wordlist on first use.
func ensureWordlistLoaded() {
	if bip39Words != nil {
		return
	}
	// Count newlines to preallocate
	count := 0
	for _, b := range bip39EnglishWordlist {
		if b == '\n' {
			count++
		}
	}
	bip39Words = make([]string, 0, count)
	bip39WordMap = make(map[string]int, count)

	start := 0
	for i, b := range bip39EnglishWordlist {
		if b == '\n' {
			word := string(bip39EnglishWordlist[start:i])
			if word != "" {
				bip39WordMap[word] = len(bip39Words)
				bip39Words = append(bip39Words, word)
			}
			start = i + 1
		}
	}
	// Handle last line if no trailing newline
	if start < len(bip39EnglishWordlist) {
		word := string(bip39EnglishWordlist[start:])
		if word != "" {
			bip39WordMap[word] = len(bip39Words)
			bip39Words = append(bip39Words, word)
		}
	}
}

// GenerateRecoveryPhrase generates a 24-word BIP-39 mnemonic from 256 bits
// of cryptographically secure entropy with an 8-bit checksum.
func GenerateRecoveryPhrase() ([]string, error) {
	ensureWordlistLoaded()

	// 256 bits = 32 bytes of entropy
	entropy := make([]byte, 32)
	if _, err := rand.Read(entropy); err != nil {
		return nil, fmt.Errorf("failed to generate entropy: %w", err)
	}

	// Compute SHA-256 checksum and take first byte (8 bits)
	checksum := sha256.Sum256(entropy)
	checksumByte := checksum[0]

	// Combine entropy + checksum: 256 + 8 = 264 bits = 33 bytes
	combined := make([]byte, 33)
	copy(combined, entropy)
	combined[32] = checksumByte

	// Split into 24 groups of 11 bits, each mapping to a word
	words := make([]string, 24)
	for i := 0; i < 24; i++ {
		// Extract 11 bits starting at bit position i*11
		bitStart := i * 11
		byteStart := bitStart / 8
		bitOffset := bitStart % 8

		// Read up to 3 bytes and extract 11 bits
		var value uint32
		bytesAvailable := len(combined) - byteStart
		if bytesAvailable > 3 {
			bytesAvailable = 3
		}
		for j := 0; j < bytesAvailable; j++ {
			value = (value << 8) | uint32(combined[byteStart+j])
		}
		// Shift to align the 11 bits to the right
		value = value >> (bytesAvailable*8 - bitOffset - 11)
		value = value & 0x7FF // mask to 11 bits

		words[i] = bip39Words[value]
	}

	return words, nil
}

// ValidateRecoveryPhrase validates a 24-word BIP-39 recovery phrase by
// recomputing and checking the checksum.
func ValidateRecoveryPhrase(words []string) error {
	ensureWordlistLoaded()

	if len(words) != 24 {
		return fmt.Errorf("%w: got %d words", ErrInvalidPhraseLength, len(words))
	}

	// Convert words to 11-bit indices
	indices := make([]uint32, 24)
	for i, word := range words {
		idx, ok := bip39WordMap[word]
		if !ok {
			return fmt.Errorf("%w: %q at position %d", ErrWordNotInList, word, i)
		}
		indices[i] = uint32(idx)
	}

	// Reconstruct the 264-bit combined value from 24 × 11-bit groups
	var combined [33]byte
	// Use a bit buffer approach
	var bitBuf uint32
	var bitCount uint
	byteIdx := 0

	for i := 0; i < 24; i++ {
		bitBuf = (bitBuf << 11) | (indices[i] & 0x7FF)
		bitCount += 11

		for bitCount >= 8 {
			bitCount -= 8
			b := uint8((bitBuf >> bitCount) & 0xFF)
			if byteIdx < 33 {
				combined[byteIdx] = b
				byteIdx++
			}
		}
	}

	// First 32 bytes are entropy, last byte's top 8 bits are checksum
	entropy := combined[:32]
	expectedChecksum := combined[32]

	// Recompute checksum
	checksum := sha256.Sum256(entropy)
	actualChecksum := checksum[0]

	if actualChecksum != expectedChecksum {
		return ErrInvalidChecksum
	}

	return nil
}


