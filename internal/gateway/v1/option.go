package v1

import (
	"log/slog"
	"net/url"
)

// Option configures a gateway handler.
type Option func(*Handler)

// WithLogger sets the logger used by the gateway handler.
func WithLogger(logger *slog.Logger) Option {
	return func(h *Handler) {
		if logger == nil {
			return
		}
		h.logger = logger.With("component", "gateway")
	}
}

// WithInvitationBaseURL sets the frontend URL used for invitation links.
func WithInvitationBaseURL(v *url.URL) Option {
	return func(h *Handler) {
		h.frontendInvitationURL = v
	}
}
