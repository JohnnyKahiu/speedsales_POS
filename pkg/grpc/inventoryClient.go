package grpc

import (
	"context"

	pb "github.com/JohnnyKahiu/speed_sales_proto/inventory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InventoryService struct {
	inventoryClient pb.InventoryServiceClient
}

func NewInventoryService(address string) (*InventoryService, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewInventoryServiceClient(conn)

	return &InventoryService{
		inventoryClient: client,
	}, nil
}

func (s *InventoryService) SearchProduct(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	resp, err := s.inventoryClient.SearchProduct(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
