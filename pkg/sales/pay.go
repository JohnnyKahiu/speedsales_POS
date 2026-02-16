package sales

type MpesaDetails struct {
	MpesaCode string  `json:"mpesa_code"`
	Amount    float64 `json:"mpesa_tendered"`
}

type ETR struct {
	Seal string `json:"seal"`
	TSIN string `json:"tsin"`
	DATE string `json:"date"`
	CUSN string `json:"cusn"`
	CUIN string `json:"cuin"`
}
