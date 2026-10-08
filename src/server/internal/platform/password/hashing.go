package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

//todo: if there is a better, more idiomatic way to handle the errors, do so
//todo: but leave that for when you have to do the actual implementation
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("Error: The following password could not be hashed; %v", err)
	}
	return string(bytes), err
}

func CheckPassword(hash, password string) (bool, error) {
	// return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	// this function couldve been a one-liner, as demonstrated above, but for sake of readability and error handling
	// it has been made into the following:

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("Error: The compared hashed password with its possible plaintext equivalent yielded an error.\nThe hashed and non-hashed passwords might be matching; %v", err)
	}
	return true, nil
}
