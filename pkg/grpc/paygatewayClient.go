package grpc

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	pb "github.com/JohnnyKahiu/speed_sales_proto/pay_gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)


// jsonCodec lets us exchange plain Go structs over gRPC without protobuf.
// It overrides the built-in "proto" name so gRPC uses it by default for
// any connection that opts in via CallContentSubtype("json").
func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return "json" }

// PayGatewayService wraps the generated gRPC client.
type PayGatewayService struct {
	client pb.PayGatewayServiceClient
}

var (
	pgwOnce sync.Once
	pgwSvc  *PayGatewayService
	pgwErr  error
)

// GetPayGatewayService returns the shared gRPC client, dialing once on first call.
// Address is read from PAYGATEWAY_GRPC_ADDR env (default "localhost:8105").
func GetPayGatewayService() (*PayGatewayService, error) {
	pgwOnce.Do(func() {
		addr := os.Getenv("PAYGATEWAY_GRPC_ADDR")
		if addr == "" {
			addr = "localhost:8105"
		}
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			pgwErr = err
			return
		}
		pgwSvc = &PayGatewayService{client: pb.NewPayGatewayServiceClient(conn)}
	})
	return pgwSvc, pgwErr
}

// STKPush proxies a push request to the PayGateway microservice over gRPC.
func (s *PayGatewayService) STKPush(req *pb.STKPushRequest) (*pb.STKPushResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.client.STKPush(ctx, req)
}
