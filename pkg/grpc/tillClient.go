package grpc

import (
	"context"

	protoUser "github.com/JohnnyKahiu/speed_sales_proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TillService struct {
	tillClient protoUser.TillServiceClient
}

// NewUserService creates a new inventory service
func NewTillService(authAddr string) (*TillService, error) {
	conn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := protoUser.NewTillServiceClient(conn)

	return &TillService{
		tillClient: client,
	}, nil
}

// UpdateTill calls Login.UpdateTill over gRPC
func (s *TillService) UpdateTill(ctx context.Context, req *protoUser.UpdateTillRequest) (*protoUser.UpdateTillResponse, error) {
	resp, err := s.tillClient.UpdateTill(ctx, req)
	if err != nil {
		return nil, err
	}

	// fmt.Println("\n\t resp = ", resp)

	return resp, nil
}
