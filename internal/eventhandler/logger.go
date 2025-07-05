package eventhandler

import (
	"log/slog"

	"github.com/open-feature/go-sdk/openfeature"
)

func CreateDroppedEventLogger(logger *slog.Logger, providerName string, args ...any) func(event openfeature.Event) {
	return func(event openfeature.Event) {
		logger.Error("Event dropped due to full channel", append(args, "provider", providerName, "event", event)...)
	}
}
