package security

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	// A2IDtime is the number of iterations (or passes) over the memory used by the algorithm Argon2id
	A2IDtime = 3

	// A2IDmemory is the amount of memory used by the algorithm Argon2id
	A2IDmemory = 64 * 1024

	// A2IDthreads is the number of threads used by the algorithm Argon2id
	A2IDthreads = 4

	// A2IDkeyLen is the length of the generated key (or password hash) by the algorithm Argon2id. 16 bytes or more is recommended.
	A2IDkeyLen = 32

	// A2IDsaltLen is the length of the random salt used by the algorithm Argon2id. 16 bytes is recommended for password hashing.
	A2IDsaltLen = 16
)

// HashPassword is used to generate a new password hash for storing and comparing at a later date.
func HashPassword(password string) (hashedPassword string, err error) {
	// Generate a cryptographically secure random salt.
	salt, err := GenerateRandomBytes(A2IDsaltLen)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, A2IDtime, A2IDmemory, A2IDthreads, A2IDkeyLen)

	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	format := "$argon2id$v=%d$models=%d,t=%d,p=%d$%s$%s"
	hashedPassword = fmt.Sprintf(format, argon2.Version, A2IDmemory, A2IDtime, A2IDthreads, b64Salt, b64Hash)
	return hashedPassword, nil
}

// ComparePassword is used to compare a user-inputted password to a hash to see if the password matches or not.
func ComparePassword(hashedPassword, password string) (match bool, err error) {
	parts := strings.Split(hashedPassword, "$")

	if len(parts) <= 5 {
		return false, errors.New("incorrectly hashed")
	}

	memory := A2IDmemory
	time := A2IDtime
	threads := A2IDthreads

	_, err = fmt.Sscanf(parts[3], "models=%d,t=%d,p=%d", &memory, &time, &threads)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	keyLen := uint32(len(decodedHash))

	comparisonHash := argon2.IDKey([]byte(password), salt, A2IDtime, A2IDmemory, A2IDthreads, keyLen)

	return (subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1), nil
}

// bcrypt.DefaultCost is a good starting point for cost (recommended between 10-14 for most applications).
func HashPasswordBcrypt(password string) (hashedPassword string, err error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// bcrypt.CompareHashAndPassword will hash the plain text password and compare it with the hashed password
func ComparePasswordBcrypt(hashedPassword, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return false, err
	}

	return true, nil
}
