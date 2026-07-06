package grpc

import (
	"context"
	"os"
	"sync"
	"time"

	pb "github.com/JohnnyKahiu/speed_sales_proto/etims"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type EtimsService struct {
	client pb.EtimsServiceClient
}

var (
	etimsOnce sync.Once
	etimsSvc  *EtimsService
	etimsErr  error
)

// GetEtimsService returns the shared gRPC client, dialing once on first call.
// Address is read from ETIMS_GRPC_ADDR env (default "localhost:50056").
func GetEtimsService() (*EtimsService, error) {
	etimsOnce.Do(func() {
		addr := os.Getenv("ETIMS_GRPC_ADDR")
		if addr == "" {
			addr = "localhost:50056"
		}
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			etimsErr = err
			return
		}
		etimsSvc = &EtimsService{client: pb.NewEtimsServiceClient(conn)}
	})
	return etimsSvc, etimsErr
}

// SubmitInvoice forwards an invoice signing request to the ETIMS microservice.
func (s *EtimsService) SubmitInvoice(ctx context.Context, req *pb.InvoiceRequest) (*pb.InvoiceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.client.SubmitInvoice(ctx, req)
}
