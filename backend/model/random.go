package model

import (
	"math/rand"
	"time"
)

var (
	values = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
)

// generateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a random string composed of ascii characters
func GenerateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	buf := make([]rune, length)

	buf[0] = values[rand.Intn(len(values))]
	for i := 1; i < length; i++ {
		buf[i] = values[rand.Intn(len(values))]
	}

	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})

	return string(buf)
}
