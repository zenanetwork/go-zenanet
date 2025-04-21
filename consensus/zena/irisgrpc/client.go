package irisgrpc

import (
	"time"

	"github.com/zenanetwork/go-zenanet/log"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	proto "github.com/zenanetwork/zenaproto/iris"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	stateFetchLimit = 50
)

type IrisGRPCClient struct {
	conn   *grpc.ClientConn
	client proto.IrisClient
}

func NewIrisGRPCClient(address string) *IrisGRPCClient {
	opts := []grpc_retry.CallOption{
		grpc_retry.WithMax(10000),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(5 * time.Second)),
		grpc_retry.WithCodes(codes.Internal, codes.Unavailable, codes.Aborted, codes.NotFound),
	}

	conn, err := grpc.NewClient(address,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Crit("Failed to connect to Iris gRPC", "error", err)
	}

	log.Info("Connected to Iris gRPC server", "address", address)

	return &IrisGRPCClient{
		conn:   conn,
		client: proto.NewIrisClient(conn),
	}
}

func (h *IrisGRPCClient) Close() {
	log.Debug("Shutdown detected, Closing Iris gRPC client")
	h.conn.Close()
}
