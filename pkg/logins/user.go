package logins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	pb "github.com/JohnnyKahiu/speed_sales_proto/user"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/grpc"
)

// FetchUser fetches user details from login server
// populates struct and reutns an error if fails
func (arg *Users) FetchUser(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	address := os.Getenv("LOGIN_RPC_ADDR")

	loginService, err := grpc.NewUserService(address)
	if err != nil {
		fmt.Println("failed to create login service    err =", err)
		return err
	}
	fmt.Println("loginService created")

	resp, err := loginService.FetchUser(ctx, &pb.UserRequest{Username: arg.Username})
	if err != nil {
		return err
	}
	fmt.Println("response =", resp)

	err = json.Unmarshal([]byte(resp.UserDetails), &arg)
	if err != nil {
		return err
	}

	return nil
}
