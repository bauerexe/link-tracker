package handlers

import (
	"net/url"
	"strings"
)

func SplitArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}

func NormalizeURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	if u.Scheme == "" {
		u, err = url.Parse("https://" + raw)
		if err != nil {
			return "", false
		}
	}

	if !u.IsAbs() || u.Host == "" {
		return "", false
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", false
	}

	return u.String(), true
}

func HasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
