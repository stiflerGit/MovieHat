package weighted

import "log/slog"

type Option func(*Lottery)

func WithLogger(l *slog.Logger) Option {
	return func(e *Lottery) {
		if l == nil {
			return
		}

		e.logger = l
	}
}
