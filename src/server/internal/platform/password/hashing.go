package password

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: The following password could not be hashed; %v", err))
	}
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	// return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	// this function couldve been a one-liner, as demonstrated above, but for sake of readability and error handling
	// it has been made into the following:

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) 
	if err != nil {
		log.Fatal(fmt.Errorf("Error: The compared hashed password with its possible plaintext equivalent yielded an error.\nThe hashed and non-hashed passwords might be matching; %v", err))
	}
	return true
}
