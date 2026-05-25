package sales

type Account struct {
	AcNum      int     `json:"ac_num"`
	AcName     string  `json:"ac_name"`
	Amount     float64 `json:"amount"`
	Supervisor string  `json:"supervisor"`
}
