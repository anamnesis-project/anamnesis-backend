package main

import (
	"strings"
)

func ContainsNumber(s string) bool {
	return strings.ContainsAny(s, "0123456789")
}

func ContainsLowerCaseLetter(s string) bool {
	return strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz")
}

func ContainsUpperCaseLetter(s string) bool {
	return strings.ContainsAny(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

func ContainsSpecialCharacter(s string) bool {
	return strings.ContainsAny(s, "!@#$%^*(),./;\\[]{}:|<>?")
}

func ValidateCPF(cpf string) bool {
	if len(cpf) != 11 {
		return false
	}

	var ints [11]int

	// ensures no non number character and gets characters as ints 
	for i := 0; i < len(cpf); i++ {
		c := cpf[i]
		if c < '0' || c > '9' {
			return false
		}

		ints[i] = int(c - '0')
	}

	sum := 0
	sum += ints[0] * 10
	sum += ints[1] * 9
	sum += ints[2] * 8
	sum += ints[3] * 7
	sum += ints[4] * 6
	sum += ints[5] * 5
	sum += ints[6] * 4
	sum += ints[7] * 3
	sum += ints[8] * 2

	firstVerifier := 11 - (sum % 11)
	if firstVerifier >= 10 {
		firstVerifier = 0
	}
	if ints[9] != firstVerifier {
		return false
	}

	sum = 0
	sum += ints[0] * 11
	sum += ints[1] * 10
	sum += ints[2] * 9
	sum += ints[3] * 8
	sum += ints[4] * 7
	sum += ints[5] * 6
	sum += ints[6] * 5
	sum += ints[7] * 4
	sum += ints[8] * 3
	sum += ints[9] * 2

	secondVerifier := 11 - (sum % 11)
	if secondVerifier >= 10 {
		secondVerifier = 0
	}
	if ints[10] != secondVerifier {
		return false
	}

	return true
}
