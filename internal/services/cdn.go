package services

import (
	"net/url"
	"strings"

	"halostory_go_backend/internal/config"
)

// CDNBase returns the public CDN base URL (from CDN_URL), with trailing slashes trimmed.
func CDNBase() string {
	return strings.TrimRight(config.Env("CDN_URL", ""), "/")
}

// ImageURL converts a stored image path (e.g. "Bot/{id}/file.png") or a full URL
// into an publicly accessible CDN URL.
func ImageURL(path string) string {
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	base := CDNBase()
	if base == "" {
		return path
	}

	return base + "/" + encodePath(path)
}

func encodePath(path string) string {
	segments := strings.Split(strings.TrimLeft(path, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}