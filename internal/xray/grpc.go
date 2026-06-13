package xray

import (
	"context"
	"fmt"
	"strings"

	"github.com/xtls/xray-core/app/proxyman/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/vless"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClient is a Client backed by a real Xray-core gRPC API endpoint.
// It uses Xray's HandlerService to alter a running inbound in place, so
// adding or removing a user never restarts the proxy or drops connections.
type GRPCClient struct {
	conn    *grpc.ClientConn
	handler command.HandlerServiceClient
	stats   statscmd.StatsServiceClient
}

// compile-time check that *GRPCClient satisfies the Client interface.
var _ Client = (*GRPCClient)(nil)

// Dial connects to the Xray gRPC API at addr (host:port). The API is expected
// to be reachable locally or over a trusted network, so the connection is not
// TLS-wrapped — securing it is the node-agent's job in the multi-node phase.
func Dial(addr string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial xray %s: %w", addr, err)
	}
	return &GRPCClient{
		conn:    conn,
		handler: command.NewHandlerServiceClient(conn),
		stats:   statscmd.NewStatsServiceClient(conn),
	}, nil
}

// AddUser adds a VLESS user to the inbound tagged inboundTag.
func (c *GRPCClient) AddUser(ctx context.Context, inboundTag, email, uuid, flow string) error {
	account := &vless.Account{
		Id:         uuid,
		Flow:       flow,
		Encryption: "none", // VLESS carries no transport encryption of its own
	}
	op := &command.AddUserOperation{
		User: &protocol.User{
			Email:   email,
			Account: serial.ToTypedMessage(account),
		},
	}
	_, err := c.handler.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       inboundTag,
		Operation: serial.ToTypedMessage(op),
	})
	if err != nil {
		return fmt.Errorf("xray add user %q: %w", email, err)
	}
	return nil
}

// RemoveUser removes the user identified by email from the inbound.
func (c *GRPCClient) RemoveUser(ctx context.Context, inboundTag, email string) error {
	op := &command.RemoveUserOperation{Email: email}
	_, err := c.handler.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag:       inboundTag,
		Operation: serial.ToTypedMessage(op),
	})
	if err != nil {
		return fmt.Errorf("xray remove user %q: %w", email, err)
	}
	return nil
}

// Stats returns per-user traffic in bytes (uplink+downlink summed), keyed by
// email. Xray exposes counters named "user>>>EMAIL>>>traffic>>>uplink" and
// "...>>>downlink"; we query them all and fold them per user.
func (c *GRPCClient) Stats(ctx context.Context, reset bool) (map[string]int64, error) {
	resp, err := c.stats.QueryStats(ctx, &statscmd.QueryStatsRequest{
		Pattern: "user>>>",
		Reset_:  reset,
	})
	if err != nil {
		return nil, fmt.Errorf("xray query stats: %w", err)
	}
	out := make(map[string]int64)
	for _, st := range resp.GetStat() {
		if email := statEmail(st.GetName()); email != "" {
			out[email] += st.GetValue()
		}
	}
	return out, nil
}

// statEmail extracts the email from a counter name of the form
// "user>>>EMAIL>>>traffic>>>uplink", or returns "" if it doesn't match.
func statEmail(name string) string {
	parts := strings.Split(name, ">>>")
	if len(parts) >= 2 && parts[0] == "user" {
		return parts[1]
	}
	return ""
}

// Close closes the underlying gRPC connection.
func (c *GRPCClient) Close() error { return c.conn.Close() }
