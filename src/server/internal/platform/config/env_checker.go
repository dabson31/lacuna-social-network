package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

func validateEnvFormat() error {

	//todo: check later if the content and division of the files is correct and appropriate
	//todo: given the structure of course

	file, err := os.Open(".env")
	if err != nil {
		log.Fatal(fmt.Errorf("Error: Could not read .env; file might not exist; %v", err))
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// ignore '#' because its a comment
		if line == "" || strings.HasPrefix(line, "#") { continue }
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 { continue }

		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		//todo check for error here:
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

//todo: helper function to read individual keys and return the value if if its not empty
//todo: else, return an empty value
func KeyValueReader(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}