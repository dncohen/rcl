package rippledata

import (
	"time"

	"github.com/rubblelabs/ripple/data"
	"github.com/y0ssar1an/q"
)

// data.ripple.com/v2/accounts/...

// {"result":"success","account_data":{"account":"rDCh8v8g2g7eGEkXWLMz2gRPe2TLbwoocB","parent":"rDsbeomae4FXwgQTJp9Rs64Qg9vDiTCdBv","initial_balance":"200","inception":"2018-04-03T17:52:20Z","ledger_index":37698948,"tx_hash":"410EFBD059677888DBF87C73253B9F97A4CD47C64A535249FA782D0CD6F603B6"}}
type AccountData struct {
	Account        data.Account
	Parent         data.Account
	InitialBalance data.Value `json:"initial_balance"`
	LedgerIndex    uint32     `json:"ledger_index"`
	Inception      time.Time
	TxHash         data.Hash256 `json:"tx_hash"`
}

type AccountResponse struct {
	Response
	AccountData AccountData `json:"account_data"`
}

func (this Client) AccountData(account data.Account) (response *AccountResponse, err error) {
	response = &AccountResponse{}
	err = this.Get(response, this.Endpoint("accounts", account.String()), nil)
	q.Q(response.raw, response, err)
	return
}
