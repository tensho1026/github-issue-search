package config_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
)

func ExampleLoad() {
	restore := clearEnvironment()
	defer restore()

	cfg, err := config.Load()
	fmt.Printf(
		"valid=%t port=%s auth=%t database=%t\n",
		err == nil,
		cfg.Port,
		cfg.AuthEnabled,
		cfg.DatabaseURL.IsSet(),
	)

	// Output:
	// valid=true port=8080 auth=false database=false
}

func clearEnvironment() func() {
	original := os.Environ()
	os.Clearenv()
	return func() {
		os.Clearenv()
		for _, item := range original {
			key, value, found := strings.Cut(item, "=")
			if !found {
				continue
			}
			if err := os.Setenv(key, value); err != nil {
				panic(err)
			}
		}
	}
}
