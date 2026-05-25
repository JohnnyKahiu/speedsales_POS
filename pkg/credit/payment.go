package credit

import "time"

type Payment struct {
	TransDate time.Time `json:"trans_date"`
	ID        int64     `json:"id"`
	TillNum   int64     `json:"till_num"`
	Cash      float64   `json:"cash"`
	Mpesa     float64   `json:"mpesa"`
	Ecard     float64   `json:"ecard"`
	Check     float64   `json:"check"`
	Total     float64   `json:"total"`
}
