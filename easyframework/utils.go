package easyframework

import "math/rand"

func GenerateSixteenDigitCode() string {
	low := 65
	high := 90

	var result [16]byte
	for i := 0; i < 16; i++ {
		char := low + rand.Intn(high-low)
		result[i] = byte(char)
	}

	return string(result[:])
}
