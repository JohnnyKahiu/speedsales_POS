package grpc

import (
	"context"
	"log"

	pb "github.com/JohnnyKahiu/speed_sales_proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthService struct {
	authClient pb.AuthServiceClient
}

// NewAuthService creates a new login_service grpc client
func NewAuthService(authAddr string) (*AuthService, error) {

	conn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewAuthServiceClient(conn)

	return &AuthService{
		authClient: client,
	}, nil
}

// ValidateUserToken calls Login.ValidateToken over gRPC
func (s *AuthService) ValidateUserToken(ctx context.Context, token string) (string, bool) {
	// ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	// defer cancel()

	resp, err := s.authClient.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: token})
	if err != nil {
		return "", false
	}

	log.Printf("resp = %v,\n", resp.Rights)
	// fmt.Printf("token = %v\n", token)

	if !resp.Valid {
		return "", false
	}

	return resp.Rights, resp.Valid
}
