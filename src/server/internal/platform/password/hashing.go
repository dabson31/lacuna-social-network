package password

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

// just some hashing for passwords with its check (even tho its in oauth)
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(fmt.Errorf("Error: The following password could not be hashed; %v", err))
	}
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) 
	if err != nil {
		log.Fatal(fmt.Errorf("Error: The compared hashed password with its possible plaintext equivalent yielded an error.\nThe hashed and non-hashed passwords might be matching; %v", err))
	}
	return true
}
