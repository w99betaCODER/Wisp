// Package xray talks to an Xray-core instance over its gRPC API to add and
// remove users from a running inbound without restarting the process.
//
// The rest of the panel depends only on the Client interface, so the no-op
// implementation (used in development) and the real gRPC implementation are
// fully interchangeable.
package xray

import (
	"context"
	"log"
)

// Client manages the set of users an Xray inbound accepts.
type Client interface {
	// AddUser adds a VLESS user to the inbound tagged inboundTag.
	AddUser(ctx context.Context, inboundTag, email, uuid, flow string) error
	// RemoveUser removes the user with the given email from the inbound.
	RemoveUser(ctx context.Context, inboundTag, email string) error
	// Stats returns per-user traffic in bytes (uplink+downlink summed),
	// keyed by email. When reset is true Xray's counters are zeroed, so each
	// call returns the delta since the previous call.
	Stats(ctx context.Context, reset bool) (map[string]int64, error)
	// Close releases any underlying connection.
	Close() error
}

// New returns a gRPC-backed Client when apiAddr is set (host:port of the Xray
// API), or a NoopClient when it is empty. protocol selects the account type
// added to the inbound ("vless", "vmess" or "trojan"). Both the panel and the
// node agent use this so behaviour is identical everywhere.
func New(apiAddr, protocol string) (Client, error) {
	if apiAddr == "" {
		return NewNoopClient(), nil
	}
	return Dial(apiAddr, protocol)
}

// NoopClient satisfies Client without contacting Xray. It logs what it would
// have done, so the panel runs end-to-end on a machine with no Xray present.
type NoopClient struct{}

// NewNoopClient returns a Client that only logs.
func NewNoopClient() *NoopClient { return &NoopClient{} }

func (NoopClient) AddUser(_ context.Context, tag, email, uuid, flow string) error {
	log.Printf("[xray:noop] AddUser tag=%s email=%s uuid=%s flow=%s", tag, email, uuid, flow)
	return nil
}

func (NoopClient) RemoveUser(_ context.Context, tag, email string) error {
	log.Printf("[xray:noop] RemoveUser tag=%s email=%s", tag, email)
	return nil
}

func (NoopClient) Stats(_ context.Context, _ bool) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func (NoopClient) Close() error { return nil }
