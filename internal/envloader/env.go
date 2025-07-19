package envloader

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// LoadDotEnv manually loads key=value pairs from a .env file into os.Environ
func LoadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("No .env file loaded from %s\n", path)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Ignore comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // malformed line
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		os.Setenv(key, value)
	}
}
