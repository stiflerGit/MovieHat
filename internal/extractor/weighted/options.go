package weighted

import "log/slog"

type Option func(*Extractor)

func WithLogger(l *slog.Logger) Option {
	return func(e *Extractor) {
		if l == nil {
			return
		}

		e.logger = l
	}
}
