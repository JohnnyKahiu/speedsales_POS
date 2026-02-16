module github.com/JohnnyKahiu/speedsales/poserver

go 1.25.4

require (
	github.com/JohnnyKahiu/speed_sales_proto v0.0.0-00010101000000-000000000000
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/gorilla/mux v1.8.1
)

require (
	github.com/pashagolub/pgxmock/v4 v4.9.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.6
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.38.2 // indirect
	golang.org/x/crypto v0.44.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/grpc v1.78.0
)

replace github.com/JohnnyKahiu/speed_sales_proto => ../proto/gen/github.com/JohnnyKahiu/speed_sales_proto
