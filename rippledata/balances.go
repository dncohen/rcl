package rippledata

import (
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

// {"result":"success","count":200,"marker":"rDCh8v8g2g7eGEkXWLMz2gRPe2TLbwoocB|20180404134951|000037718341|00040|$","balance_changes":[{"amount_change":"200","final_balance":"200","node_index":0,"tx_index":1,"change_type":"payment_destination","currency":"XRP","executed_time":"2018-04-03T17:52:20Z","ledger_index":37698948,"tx_hash":"410EFBD059677888DBF87C73253B9F97A4CD47C64A535249FA782D0CD6F603B6"},{"amount_change":"-0.000012","final_balance":"199.999988","tx_index":26,"change_type":"transaction_cost","currency":"XRP","executed_time":"2018-04-03T18:23:02Z","ledger_index":37699466,"tx_hash":"473BD6802D3488210522F59F79FFCBD1BA94EC1567C23F66519AF5A1225066E3"},{"amount_change":"-0.000012","final_balance":"199.999976","tx_index":27,"change_type":"transaction_cost","currency":"XRP","executed_time":"2018-04-03T18:23:02Z","ledger_index":37699466,"tx_hash":"62DE15AC37723CD93624C25BCAD6A86A2D35C461A04C0CC6C222218EC7BF02F9"},{"amount_change":"-0.000012","final_balance":"199.999964","tx_index":28,"change_type":"transaction_cost","currency":"XRP","executed_time":"2018-04-03T18:23:02Z","ledger_index":37699466,"tx_hash":"369F97D567F5677D10F1512C5AB098B044F15FA9C12BED600546AC1522F783B1"},{"amount_change":"-0.000012","final_balance":"199.999952","tx_index":29,"change_type":"transaction_cost","currency":"XRP","executed_time":"2018-04-03T18:23:02Z","ledger_index":37699466,"tx_hash":"CC0C91F08774B494790591FB1D4759D474BAA87362EAE805BF0E8D387BAB331E"},{"amount_change":"-28.571428","final_balance":"171.428524","node_index":5,"tx_index":17,"change_type":"exchange","currency":"XRP","executed_time":"2018-04-03T18:37:40Z","ledger_index":37699705,"tx_hash":"6F347A4ABABFD2CA139131D1303C36D9415381ED629E479AADE3A91900B88C80"},{"amount_change":"15.218875","final_balance":"15.218875","node_index":13,"tx_index":17,"change_type":"exchange","counterparty":"rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B","currency":"USD","executed_time":"2018-04-03T18:37:40Z","ledger_index":37699705,"tx_hash":"6F347A4ABABFD2CA139131D1303C36D9415381ED629E479AADE3A91900B88C80"},

// https://ripple.com/build/data-api-v2/#balance-change-descriptors
type BalanceChangeDescriptor struct {
	TransactionResponse

	AmountChange data.NonNativeValue `json:"amount_change"`
	FinalBalance data.NonNativeValue `json:"final_balance"`
	ChangeType   string              `json:"change_type"`
	Currency     string              `json:"currency"`
	Counterparty data.Account        `json:"counterparty"`

	TxIndex uint32 `json:"tx_index"`
	//LedgerIndex  uint32              `json:"ledger_index"`
	//TxHash       data.Hash256        `json:"tx_hash"`
	//ExecutedTime time.Time           `json:"executed_time"`
}

func (change BalanceChangeDescriptor) GetTransactionIndex() uint32 {
	return change.TxIndex
}

func (change BalanceChangeDescriptor) GetFinalAmount() *data.Amount {
	str := fmt.Sprintf("%s/%s/%s", change.FinalBalance, change.Currency, change.Counterparty)
	amt, err := data.NewAmount(str)
	if err != nil {
		log.Panic(err)
	}
	return amt
}
func (change BalanceChangeDescriptor) GetChangeAmount() *data.Amount {
	str := fmt.Sprintf("%s/%s/%s", change.AmountChange, change.Currency, change.Counterparty)
	amt, err := data.NewAmount(str)
	if err != nil {
		log.Panic(err)
	}
	return amt

}

type BalanceChangesResponse struct {
	Response
	Count          int                       `json:"count"`
	Marker         string                    `json:"marker"`
	BalanceChanges []BalanceChangeDescriptor `json:"balance_changes"`
}

func (this Client) GetBalanceChanges(account data.Account, marker string) (response *BalanceChangesResponse, err error) {
	response = &BalanceChangesResponse{}
	endpoint := this.Endpoint("accounts", account.String(), "balance_changes")
	values := endpoint.Query()
	if marker != "" {
		values.Set("marker", marker)
	}
	err = this.Get(response, endpoint, &values)

	//q.Q(response, err)
	return
}

func (this Client) GetBalanceChangesAsync(account data.Account) chan BalanceChangeDescriptor {
	c := make(chan BalanceChangeDescriptor) // TODO buffer

	go func() {
		done := false
		marker := ""
		consecutiveErrors := 0
		for !done {
			//log.Printf("BalanceChangesAsync: Requesting balance changes for %s with marker %s", account, marker) // debug
			response, err := this.GetBalanceChanges(account, marker)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= 100 {
					log.Panic(errors.Wrapf(err, "rippledata: failed to get balances changes, %d attempt(s)", consecutiveErrors))
				}
				<-time.After(time.Second * time.Duration(consecutiveErrors)) // Wait before trying again.
				log.Println(errors.Wrapf(err, "rippledata: failed to get balances changes, %d attempt(s)", consecutiveErrors))
				continue // try again
			} else {
				if consecutiveErrors > 0 {
					log.Printf("rippledata: success after %d earlier failed attempts\n", consecutiveErrors)
				}
				consecutiveErrors = 0
			}
			for _, change := range response.BalanceChanges {
				c <- change
			}
			if response.Marker == "" {
				//log.Printf("BalanceChangesAsync: No more balance changes for %s", account) // debug
				done = true
			}
			marker = response.Marker
		}
		close(c)
	}()

	return c
}

/* {
  "result": "success",
  "amount": "100",
  "converted": "0.4267798022744489",
  "rate": "0.0042677980"
}*/
type NormalizeResponse struct {
	Response
	Amount    data.NonNativeValue `json:"amount"`
	Converted data.NonNativeValue `json:"converted"`
	Rate      data.NonNativeValue `json:"rate"`
}

func (this Client) Normalize(amount data.Amount, normalizeTo data.Asset, when time.Time) (*NormalizeResponse, error) {
	response := &NormalizeResponse{}
	endpoint := this.Endpoint("normalize")
	values := endpoint.Query()

	values.Set("amount", amount.Value.Abs().String())
	values.Set("currency", amount.Currency.String())
	if amount.Currency.String() != "XRP" {
		values.Set("issuer", amount.Issuer.String())
	}

	values.Set("exchange_currency", normalizeTo.Currency)
	if normalizeTo.Currency != "XRP" {
		values.Set("exchange_issuer", normalizeTo.Issuer)
	}

	values.Set("date", when.UTC().Format(time.RFC3339))

	err := this.Get(response, endpoint, &values)
	//q.Q(string(response.raw), err)
	return response, err
}
