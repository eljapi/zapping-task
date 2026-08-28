package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

/*
72 is not our choice: bcrypt hashes only the first 72 bytes and silently ignores
the rest, so anything longer would make the tail of the password meaningless
*/
const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
)

var ErrInvalidCredentials = errors.New("invalid email or password")

/*
Fuel for burnTime, never a credential: no account authenticates against it, so
it is generated at boot instead of pasted as a literal. bcrypt writes the cost
into the hash string itself, so a hardcoded one silently stops matching the
cost HashPassword uses the day that cost changes, and the timing gap comes back
wider than it was before the mitigation. The salt is different on every start
and that changes nothing: bcrypt always runs 2^cost iterations, so what the
comparison costs depends on the cost alone
*/
var dummyHash = newDummyHash()

func newDummyHash() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("timing equaliser"), bcrypt.DefaultCost)
	if err != nil {
		/*
			GenerateFromPassword only fails on a cost out of range or an input over
			72 bytes, both of them bugs in this file rather than runtime conditions.
			A binary that cannot equalise login timing should not start
		*/
		panic("generating the dummy bcrypt hash: " + err.Error())
	}

	return hash
}

/*
The salt is generated inside GenerateFromPassword and stored in the resulting
string, together with the cost, so the hash carries everything needed to verify it
*/
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

/*
Any comparison failure collapses into the same error: telling apart a bad hash
from a wrong password would only help an attacker
*/
func CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	return nil
}

/*
An unknown email would otherwise answer immediately while a wrong password spends
~40ms in bcrypt, and that gap alone tells an attacker which addresses exist.
This runs a comparison it knows will fail, just to pay the same cost
*/
func burnTime(password string) {
	bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
