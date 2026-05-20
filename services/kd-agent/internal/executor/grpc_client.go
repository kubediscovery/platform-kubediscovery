package executor

import (
	"context"
	"fmt"

	executorv1 "github.com/kubediscovery/kd-libs/core/v1/executor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// GRPCClient implements Client by connecting to the kd-executor gRPC service.
//
// kd-executor must expose a gRPC endpoint that accepts ExecutorCommand messages.
// The agent dials this endpoint on every Execute call (connection pooling is
// delegated to the gRPC ClientConn).
//
// When the executor endpoint is unreachable — e.g. because kd-executor is not
// deployed or has not started yet — Execute returns ErrUnavailable so that the
// caller can forward an appropriate error to kd-gateway.
type GRPCClient struct {
	addr    string
	dialOps []grpc.DialOption
}

// NewGRPCClient returns a GRPCClient that dials the kd-executor service at addr.
// Provide grpc.WithTransportCredentials(...) in opts for mTLS; the default uses
// insecure transport (suitable for same-pod communication within the Data Plane).
func NewGRPCClient(addr string, opts ...grpc.DialOption) *GRPCClient {
	if len(opts) == 0 {
		opts = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	return &GRPCClient{addr: addr, dialOps: opts}
}

// Execute dials kd-executor, forwards cmd, and returns the ExecutorResponse.
//
// It returns ErrUnavailable when:
//   - the TCP connection to the executor address cannot be established
//   - the remote end responds with gRPC codes.Unavailable
//
// All other gRPC status errors are returned as-is.
func (c *GRPCClient) Execute(ctx context.Context, cmd *executorv1.ExecutorCommand) (*executorv1.ExecutorResponse, error) {
	conn, err := grpc.NewClient(c.addr, c.dialOps...)
	if err != nil {
		return nil, fmt.Errorf("%w: dial %s: %v", ErrUnavailable, c.addr, err)
	}
	defer conn.Close()

	resp, err := callExecutor(ctx, conn, cmd)
	if err != nil {
		if isUnavailable(err) {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return nil, err
	}
	return resp, nil
}

// isUnavailable returns true for errors that map to kd-executor being
// unreachable or temporarily down.
func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	code := status.Code(err)
	return code == codes.Unavailable
}

// callExecutor is the stub that will invoke the actual ExecutorService RPC once
// the proto service definition is generated. Until the executor.proto defines a
// service, this function returns ErrUnavailable so that the unavailability path
// is always exercised and fully tested.
//
// Replace this stub with the real generated client call when ExecutorService is
// added to executor.proto and proto-gen is executed.
func callExecutor(_ context.Context, _ *grpc.ClientConn, _ *executorv1.ExecutorCommand) (*executorv1.ExecutorResponse, error) {
	// Placeholder: ExecutorService RPC not yet defined in proto.
	// Returning Unavailable causes Execute to surface ErrUnavailable upstream.
	return nil, status.Error(codes.Unavailable, "ExecutorService not yet implemented")
}
