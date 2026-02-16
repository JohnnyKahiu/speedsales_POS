package grpc

import (
	"context"

	protoUser "github.com/JohnnyKahiu/speed_sales_proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserService struct {
	userClient protoUser.UserServiceClient
}

// NewUserService creates a new inventory service
func NewUserService(authAddr string) (*UserService, error) {
	conn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := protoUser.NewUserServiceClient(conn)

	return &UserService{
		userClient: client,
	}, nil
}

// FetchUser calls Login.FetchUser over gRPC
func (s *UserService) FetchUser(ctx context.Context, req *protoUser.UserRequest) (*protoUser.UserResponse, error) {
	resp, err := s.userClient.FetchUser(ctx, req)
	if err != nil {
		return nil, err
	}

	// fmt.Println("\n\t resp = ", resp)

	return resp, nil
}
