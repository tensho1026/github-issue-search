package user

import (
	"strings"
	"testing"
)

func FuzzParseUsername(fuzzer *testing.F) {
	for _, seed := range []string{
		"octocat",
		"tensho1026",
		"a",
		"with-hyphen",
		"-leading",
		"double--hyphen",
		"日本語",
		"",
	} {
		fuzzer.Add(seed)
	}

	fuzzer.Fuzz(func(t *testing.T, raw string) {
		username, err := ParseUsername(raw)
		if err != nil {
			return
		}
		value := username.String()
		if value == "" ||
			len(value) > maximumUsernameLength ||
			strings.HasPrefix(value, "-") ||
			strings.HasSuffix(value, "-") ||
			strings.Contains(value, "--") {
			t.Fatalf("accepted invalid username %q", value)
		}
		roundTrip, roundTripErr := ParseUsername(value)
		if roundTripErr != nil || roundTrip != username {
			t.Fatalf(
				"ParseUsername(%q) round trip = %q, %v",
				value,
				roundTrip,
				roundTripErr,
			)
		}
	})
}
