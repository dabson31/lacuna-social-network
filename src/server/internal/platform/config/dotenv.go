package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

func loadDotEnv(path string) error {

	f, err := os.Open(path)

	// .env missing is not an error because, in docker, env vars come from docker-compose, not the file itself
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("Error: open %s: %v", path, err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// this only removes the first instance of '=', meaning values with '=' will pass normally
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("Error: possible malformed line; %s %v\n%v", line, lineNo, err)
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		// here, we're using LookUpEnv instead of Getenv because the former returns the value itself
		// along with a boolean that indicates whether its set. it allows to differntiate between an unset var
		// and an empty one. Getenv only returns the value of the variable, it retuns nothing if its unset
		_, exists := os.LookupEnv(key)
		if exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("Error: line (%v) has an error; %v", lineNo, err)
		}
	}

	return scanner.Err()
}
