package helper

import (
	"math/rand"
	"strconv"
	"time"
)

func GenerateRandomNumber(prefix string, digitNumber int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Calculate the range for the random number based on the number of digits
	min := int64(1)
	max := int64(10)
	for i := 1; i < digitNumber; i++ {
		min *= 10
		max *= 10
	}

	// Generate a random number with the specified number of digits
	randomNumber := prefix + "-" + strconv.FormatInt(min+r.Int63n(max-min), 10)

	return randomNumber
}

func GenerateRandomString(perfix string, length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range length {
		randomIndex := r.Intn(len(characters))
		result[i] = characters[randomIndex]
	}
	return perfix + string(result)
}
