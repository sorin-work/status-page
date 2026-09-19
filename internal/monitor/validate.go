package monitor

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits applied when validating monitors.
const (
	MaxNameLength  = 100
	MinInterval    = 10 * time.Second
	MaxInterval    = 24 * time.Hour
	MinTimeout     = time.Second
	MaxTimeout     = 60 * time.Second
	MaxRetries     = 10
	MaxKeywordSize = 200
)

// ValidationError maps field names to a human readable problem.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+v[k])
	}
	return "invalid monitor: " + strings.Join(parts, "; ")
}

// Normalize trims user input in place.
func (m *Monitor) Normalize() {
	m.Name = strings.TrimSpace(m.Name)
	m.Target = strings.TrimSpace(m.Target)
	m.Keyword = strings.TrimSpace(m.Keyword)
	m.Type = Type(strings.ToLower(strings.TrimSpace(string(m.Type))))
}

// Validate checks the monitor settings. It returns a ValidationError
// describing every invalid field, or nil.
func (m *Monitor) Validate() error {
	errs := ValidationError{}

	switch {
	case m.Name == "":
		errs["name"] = "is required"
	case utf8.RuneCountInString(m.Name) > MaxNameLength:
		errs["name"] = fmt.Sprintf("must be at most %d characters", MaxNameLength)
	}

	if !m.Type.Valid() {
		errs["type"] = "must be http or tcp"
	} else if msg := validateTarget(m.Type, m.Target); msg != "" {
		errs["target"] = msg
	}

	switch {
	case m.Interval < MinInterval:
		errs["interval"] = fmt.Sprintf("must be at least %s", MinInterval)
	case m.Interval > MaxInterval:
		errs["interval"] = fmt.Sprintf("must be at most %s", MaxInterval)
	}

	switch {
	case m.Timeout < MinTimeout:
		errs["timeout"] = fmt.Sprintf("must be at least %s", MinTimeout)
	case m.Timeout > MaxTimeout:
		errs["timeout"] = fmt.Sprintf("must be at most %s", MaxTimeout)
	case m.Timeout >= m.Interval:
		errs["timeout"] = "must be shorter than the interval"
	}

	if m.Retries < 0 || m.Retries > MaxRetries {
		errs["retries"] = fmt.Sprintf("must be between 0 and %d", MaxRetries)
	}

	if m.Type == TypeTCP {
		if m.ExpectedStatus != 0 {
			errs["expected_status"] = "only applies to HTTP monitors"
		}
		if m.Keyword != "" {
			errs["keyword"] = "only applies to HTTP monitors"
		}
		if m.NoRedirects {
			errs["follow_redirects"] = "only applies to HTTP monitors"
		}
	} else {
		if m.ExpectedStatus != 0 && (m.ExpectedStatus < 100 || m.ExpectedStatus > 599) {
			errs["expected_status"] = "must be a valid HTTP status code"
		}
		if len(m.Keyword) > MaxKeywordSize {
			errs["keyword"] = fmt.Sprintf("must be at most %d bytes", MaxKeywordSize)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateTarget(t Type, target string) string {
	if target == "" {
		return "is required"
	}
	switch t {
	case TypeHTTP:
		u, err := url.Parse(target)
		if err != nil {
			return "is not a valid URL"
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "must start with http:// or https://"
		}
		if u.Host == "" {
			return "must include a host"
		}
	case TypeTCP:
		if strings.Contains(target, "://") {
			return "must be host:port without a scheme"
		}
		if strings.ContainsAny(target, " /") {
			return "must be host:port"
		}
		host, port, err := net.SplitHostPort(target)
		if err != nil || host == "" {
			return "must be host:port, e.g. db.internal:5432"
		}
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "must have a port between 1 and 65535"
		}
	}
	return ""
}
