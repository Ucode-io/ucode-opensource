package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func Encrypt(body any, key string) (string, error) {
	// Convert the body to JSON bytes
	respByte, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	keyBytes := []byte(key)
	const keyLength = 32
	if len(keyBytes) < keyLength {
		return "", fmt.Errorf("key must be at least 32 bytes long")
	} else if len(keyBytes) > keyLength {
		keyBytes = keyBytes[:keyLength]
	}

	// Create a new AES cipher block
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	// Pad the plaintext to be a multiple of the block size
	textBytes := pkcs7Padding(respByte, aes.BlockSize)

	// Create ciphertext buffer and generate IV
	cipherText := make([]byte, aes.BlockSize+len(textBytes))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// Encrypt using CBC mode
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[aes.BlockSize:], textBytes)

	// Return the base64-encoded IV and ciphertext
	return fmt.Sprintf("%s:%s", base64.StdEncoding.EncodeToString(iv), base64.StdEncoding.EncodeToString(cipherText[aes.BlockSize:])), nil
}

// PKCS#7 padding function
func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func Decrypt(encryptedText, key string) (string, error) {
	// Split the encrypted text into IV and ciphertext
	parts := strings.Split(encryptedText, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid input format. Expected 'IV:ciphertext'")
	}

	// Decode the base64-encoded IV and ciphertext
	iv, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}

	cipherText, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	// Ensure the key is 32 bytes long
	keyBytes := []byte(key)
	const keyLength = 32
	if len(keyBytes) > keyLength {
		keyBytes = keyBytes[:keyLength]
	} else if len(keyBytes) < keyLength {
		padding := make([]byte, keyLength-len(keyBytes))
		keyBytes = append(keyBytes, padding...)
	}

	// Create a new AES cipher block
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	if len(iv) != aes.BlockSize {
		return "", fmt.Errorf("invalid IV length")
	}

	// CBC mode requires the ciphertext to be a multiple of the block size
	if len(cipherText)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	// Decrypt the ciphertext using CBC mode
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherText, cipherText)

	// Unpad the decrypted text (if needed, based on padding scheme)
	return sanitizeString(string(cipherText)), nil
}

func sanitizeString(str string) string {
	// Remove non-printable ASCII characters
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 { // Printable ASCII is between 32 and 126
			return -1
		}
		return r
	}, str)
}
