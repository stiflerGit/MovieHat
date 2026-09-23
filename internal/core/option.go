package v1

import "log/slog"

// Option configures a MovieHat handler.
type Option func(*Handler)

// WithLogger sets the logger used by the MovieHat handler.
func WithLogger(l *slog.Logger) Option {
	return func(h *Handler) {
		if l == nil {
			return
		}
		h.logger = l
	}
}
