package app

import (
	"regexp"
	"strings"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func validUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func validPassword(password string) bool {
	return len(password) >= 12 && len(password) <= 72
}
