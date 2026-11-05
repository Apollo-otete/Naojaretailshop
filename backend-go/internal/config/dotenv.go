package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadDotenv reads a .env from the working directory if one exists, without overriding
// variables the environment already sets.
//
// It is exported because both binaries need it. The migrate tool deliberately does not run
// Validate — a migration tool that refuses to work because a CORS origin list is missing is a
// migration tool that gets skipped — but it still has to find the same DATABASE_URL the API
// does, or `make migrate-up` fails for anyone who followed .env.example.
func LoadDotenv(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil // absence is normal: containers inject variables directly
	}
	// godotenv does not overwrite by default, so an explicit export always wins.
	return godotenv.Load(path)
}
