package auth

import "log/slog"

// Option configures an authentication handler.
type Option func(*Handler)

// WithLogger sets the logger used by the authentication handler.
func WithLogger(logger *slog.Logger) Option {
	return func(h *Handler) {
		if logger == nil {
			return
		}
		h.logger = logger.With("component", "auth")
	}
}
