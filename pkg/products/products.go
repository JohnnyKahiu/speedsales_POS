package products

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	pb "github.com/JohnnyKahiu/speed_sales_proto/inventory"
	"github.com/JohnnyKahiu/speedsales/poserver/pkg/grpc"
)

type StockMaster struct {
	ItemCode         string  `json:"item_code" `
	ItemName         string  `json:"item_name" `
	ItemCost         float64 `json:"item_cost" `
	ItemSellingprice float64 `json:"item_sellingprice" `
	ItemPrice        float64 `json:"item_price" `
	OnOffer          bool    `json:"on_offer" `
	KgWeight         float64 `json:"kg_weight" `
	TillPrice        float64 `json:"till_price" `
	VatPercent       float64 `json:"vat_percent" `
	Image            string  `json:"image" `
	PkgQty           float64 `json:"pkg_qty"`
	Disc             float64 `json:"Disc" `
	Label            string  `json:"label" `
	Bal              float64 `json:"bal" `
}

// Fetch gets stock data from inventory service
// Returns an error if it fails
func (p *StockMaster) Fetch(ctx context.Context) error {
	address := os.Getenv("INVENTORY_RPC_ADDR")
	inventoryService, err := grpc.NewInventoryService(address)
	if err != nil {
		return err
	}
	log.Println("inventory service created")

	resp, err := inventoryService.SearchProduct(ctx, &pb.SearchRequest{
		QueryString: fmt.Sprintf(`{"item_code": "%s"}`, p.ItemCode),
	})
	if err != nil {
		return err
	}

	if err = json.Unmarshal([]byte(resp.Result), &p); err != nil {
		return err
	}

	return nil
}
