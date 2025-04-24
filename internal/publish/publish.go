package publish

import (
	"encoding/json"
	"log/slog"

	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
)

// ErrorMessage publish error message
func ErrorMessage(conn *nats.Conn, subj string, msg string, err error) {
	response, err := json.Marshal(plexus.ErrorResponse{
		Message: msg,
		Error:   err,
	})
	if err != nil {
		slog.Error("invalid message respone", "error", err)
	}
	if err := conn.Publish(subj, response); err != nil {
		slog.Error("publish error", "error", err)
	}
}

// Message json encode and publish message
func Message(conn *nats.Conn, subj string, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("invalid message data", "error", err, "data", data)
	}
	if err := conn.Publish(subj, bytes); err != nil {
		slog.Error("publish msg", "connection", conn.Opts.Name, "subject", subj, "data", data)
	}
}
