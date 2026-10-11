package realtime

import (
	"encoding/json"
	"fmt"
)

// envelop is one connection that carries chat, notifications, presence
// instead of creating a separate WS connection for every feature, we use one connection and put a type label on each message
// meaning one socket can get: {"type":"dm.message","payload":{...}}
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// the only two type strings that belong to the transport rather than to a feature.
// dm.message and notification.new live with chat and notifications
const (
	TypePing = "ping"
	TypePong = "pong"
)

// Encode marshals a typed frame in one step
// features call this instead of building the envelope
func Encode(typ string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode %s payload: %w", typ, err)
	}
	frame, err := json.Marshal(Envelope{Type: typ, Payload: raw})
	if err != nil {
		return nil, fmt.Errorf("encode %s frame: %w", typ, err)
	}
	return frame, nil
}

// Decode reads the discriminator off an inbound frame and leaves the payload sealed for
// whichever feature handles that type.
func Decode(frame []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(frame, &e); err != nil {
		return Envelope{}, fmt.Errorf("decode frame: %w", err)
	}
	return e, nil
}
