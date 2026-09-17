package types

import (
	"context"
	"os"

	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
)

// ShouldStream returns true if SSE streaming is enabled for this job.
// Global kill-switch: USE_STREAMING=false disables all streaming.
func ShouldStream(isStream bool) bool {
	return isStream && os.Getenv("USE_STREAMING") != "false"
}

// StreamWithSanitizer wraps a stream call with a StreamSanitizer that
// publishes sanitized deltas via PublishStream and signals PublishStreamDone on success.
// The caller provides streamFn which should call onDelta for each raw delta.
// Returns true if streaming succeeded (caller should use the stream response);
// false if streaming failed and caller should fall back to non-stream.
func StreamWithSanitizer(
	ctx context.Context,
	uuid string,
	streamFn func(ctx context.Context, onDelta func(string) error) error,
) bool {
	sanitizer := &byok.StreamSanitizer{}
	err := streamFn(ctx, func(delta string) error {
		if clean := sanitizer.Feed(delta); clean != "" {
			_ = services.PublishStream(uuid, clean)
		}
		return nil
	})
	if err != nil {
		return false
	}
	if rest := sanitizer.Flush(); rest != "" {
		_ = services.PublishStream(uuid, rest)
	}
	_ = services.PublishStreamDone(uuid)
	return true
}
