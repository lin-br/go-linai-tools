package configs

import (
	"bufio"
	"os"
	"strings"
)

const envFilePath = "./.env"

// loadDotEnv parses a KEY=VALUE file and sets each entry in the process
// environment, but only when that key is not already present. Real shell
// exports and CI-injected secrets therefore always win over the file.
//
// A missing file is not an error: callers may run without a .env by relying
// solely on the ambient environment. Malformed lines (no '=') are skipped so
// a single bad entry does not abort configuration of the whole app.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		if n := len(value); n >= 2 {
			first, last := value[0], value[n-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : n-1]
			}
		}

		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
