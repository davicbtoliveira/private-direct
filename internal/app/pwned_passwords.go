package app

import (
	"bufio"
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) passwordBreached(ctx context.Context, password string) (bool, bool) {
	if s.cfg.PwnedPasswordsURL == "" {
		return false, false
	}

	digest := fmt.Sprintf("%X", sha1.Sum([]byte(password)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(s.cfg.PwnedPasswordsURL, "/")+"/"+digest[:5], nil)
	if err != nil {
		return false, false
	}
	req.Header.Set("Add-Padding", "true")

	res, err := s.httpClient.Do(req)
	if err != nil {
		return false, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false, false
	}

	suffix := digest[5:]
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		candidate, _, ok := strings.Cut(scanner.Text(), ":")
		if ok && strings.EqualFold(candidate, suffix) {
			return true, true
		}
	}
	if scanner.Err() != nil {
		return false, false
	}
	return false, true
}
