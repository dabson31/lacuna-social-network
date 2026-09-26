package logger

import (
	"log/slog"
	"os"
)

//! https://youtu.be/_mqB7LLgwgs?si=Nt7j-0g9xQqiExKJ
//? Go’s log/slog is the standard library’s structured logging package for Go 1.21,
//? where each log record includes a time, a severity level, a message, and key-value attributes.
//* Usage: basically, when someone registers, logs in, or when a WebSocket handler fails or panics,
//* there are no logs currently to tell what happened.

//! Obsidian note:
// Slog splits up the operation into two tasks: *Logger* and *Handler*.
// The Logger calls the rest of the code, *log.Info(...)* and *log.Error(...)*, for example.
// It doesn't yet look at the output; it only concerns itself with the Record;
// it packs up information to display later in the output.
// The *Handler*, on the other hand, is responsible for processing log records, formatting them,
// and writing them to output streams.
// It acts as the backend that executes the final logging actions based on the configurations set in the logger.

// Functionally speaking, New returns a logger that mains passes it down to everything else.

func New(env string) *slog.Logger {
	if env == "production" {
		// if its in production, we get a JSON, otherise return terminal readable logs
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

}
