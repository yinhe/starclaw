package sdk

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// RPCHandler processes an RPC request and returns a response or error.
type RPCHandler func(data []byte) (interface{}, error)

// rpcResponse is the wire format for RPC replies.
type rpcResponse struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// HandleRPC registers a handler for pheromone.rpc.{service}.{method}.
// Multiple instances of the same service share load via queue group.
func (c *Client) HandleRPC(method string, handler RPCHandler) error {
	subject := fmt.Sprintf("pheromone.rpc.%s.%s", c.info.Name, method)
	queue := c.info.Name + ".rpc"

	sub, err := c.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		result, err := handler(msg.Data)

		var resp rpcResponse
		if err != nil {
			resp.Error = err.Error()
		} else {
			raw, _ := json.Marshal(result)
			resp.Data = raw
		}

		reply, _ := json.Marshal(resp)
		_ = msg.Respond(reply)
	})
	if err != nil {
		return fmt.Errorf("pheromone: handle rpc %s: %w", subject, err)
	}
	c.addSub(sub)
	return nil
}

// Call makes an RPC call to pheromone.rpc.{service}.{method}.
// Returns the response data or an error (including remote errors).
func (c *Client) Call(service, method string, data interface{}, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	subject := fmt.Sprintf("pheromone.rpc.%s.%s", service, method)
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("pheromone: marshal rpc request: %w", err)
	}

	msg, err := c.nc.Request(subject, payload, timeout)
	if err != nil {
		return nil, fmt.Errorf("pheromone: rpc %s: %w", subject, err)
	}

	var resp rpcResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return msg.Data, nil // raw response
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("pheromone: rpc %s error: %s", subject, resp.Error)
	}
	return resp.Data, nil
}
