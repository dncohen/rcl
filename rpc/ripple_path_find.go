package rpc

import "github.com/rubblelabs/ripple/data"

type PathFind struct {
	Destination_account string      `json:"destination_account"`
	Destination_amount  data.Amount `json:"destination_amount"`
}
