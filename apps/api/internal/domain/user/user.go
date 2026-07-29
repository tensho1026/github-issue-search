package user

import (
	"errors"
	"strings"
)

const maximumUsernameLength = 39

var ErrInvalidUsername = errors.New("invalid GitHub username")

// Username is a validated GitHub login identifier.
type Username string

func ParseUsername(raw string) (Username, error) {
	value := strings.TrimSpace(raw)
	if len(value) < 1 || len(value) > maximumUsernameLength ||
		value[0] == '-' || value[len(value)-1] == '-' {
		return "", ErrInvalidUsername
	}

	previousHyphen := false
	for _, character := range value {
		isHyphen := character == '-'
		isLowercaseLetter := character >= 'a' && character <= 'z'
		isUppercaseLetter := character >= 'A' && character <= 'Z'
		isDigit := character >= '0' && character <= '9'
		if !isHyphen &&
			!isLowercaseLetter &&
			!isUppercaseLetter &&
			!isDigit {
			return "", ErrInvalidUsername
		}
		if isHyphen && previousHyphen {
			return "", ErrInvalidUsername
		}
		previousHyphen = isHyphen
	}

	return Username(value), nil
}

func (u Username) String() string {
	return string(u)
}

// Profile is the normalized GitHub user data used by IssueScout.
type Profile struct {
	Login       Username
	Name        string
	AvatarURL   string
	Bio         string
	PublicRepos int
	Followers   int
	Following   int
}
