package insights

import "time"

type Transaction struct {
	ID             string
	Name           string
	Classification string // income|expense
	AmountText     string // e.g. "€1.00" or "-€2.00"
	Currency       string
	Date           time.Time
	AccountName    string
	MerchantName   string
}
