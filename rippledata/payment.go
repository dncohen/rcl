package rippledata

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/y0ssar1an/q"
)

// https://ripple.com/build/data-api-v2/#balance-objects-and-balance-change-objects
type BalanceChangeObject struct {
	Counterparty string              `json:"counterparty"` // string not data.Account, because data api will return "counterparty": "" when "currency": "XRP"
	Currency     string              `json:"currency"`
	Value        data.NonNativeValue `json:"value"`
}

func (this BalanceChangeObject) GetAmount() *data.Amount {
	str := fmt.Sprintf("%s/%s", this.Value, this.Currency)
	if this.Currency != "XRP" {
		str = fmt.Sprintf("%s/%s", str, this.Counterparty)
	}
	amount, err := data.NewAmount(str)
	if err != nil {
		log.Panic(errors.Wrapf(err, "Bad amount %s", str))
	}
	return amount
}

type AccountPaymentsResponse struct {
	Response
	Count    int    `json:"count"`
	Marker   string `json:"marker"`
	Payments []Payment
}

// https://ripple.com/build/data-api-v2/#payment-objects
type Payment struct {
	TransactionResponse

	DeliveredAmount           data.NonNativeValue   `json:"delivered_amount"`
	Destination               data.Account          `json:"destination"`
	DestinationTag            *int                  `json:"destination_tag,omitempty"`
	DestinationCurrency       string                `json:"currency"`
	DestinationBalanceChanges []BalanceChangeObject `json:"destination_balance_changes"`
	Source                    data.Account          `json:"source"`
	SourceTag                 *int                  `json:"source_tag,omitempty"`
	SourceCurrency            string                `json:"source_currency"`
	SourceBalanceChanges      []BalanceChangeObject `json:"source_balance_changes"`
}

func (this Client) GetAccountPayments(account data.Account, marker string) (response *AccountPaymentsResponse, err error) {
	response = &AccountPaymentsResponse{}
	endpoint := this.Endpoint("accounts", account.String(), "payments")
	values := endpoint.Query()
	if marker != "" {
		values.Set("marker", marker)
	}
	err = this.Get(response, endpoint, &values)
	if err != nil {
		q.Q(err, string(response.raw))
	}
	return
}

func (this Client) GetAccountPaymentsAsync(account data.Account) chan Payment {
	c := make(chan Payment) // TODO buffer

	go func() {
		done := false
		marker := ""
		for !done {
			//log.Printf("GetAccountPaymentsAsync: Requesting payments for %s with marker %s", account, marker) // debug
			response, err := this.GetAccountPayments(account, marker)
			if err != nil {
				log.Panic(errors.Wrapf(err, "Failed to GetAccountPayments(%s, %s)", account, marker))
			}
			for _, event := range response.Payments {
				// debug!!! XXX
				if event.GetHash().String() == "D7E2F5A06E165B846BF1E947329414C272CD121CC3CCD549F4419C64EB870455" {
					q.Q("XXX", event.GetHash(), string(response.raw))
					//log.Fatalf("XXX GetAccountPaymentsAsync %s", event.GetHash())
				}

				c <- event
			}
			if response.Marker == "" {
				log.Printf("GetAccountPaymentsAsync: No more payments for %s", account) // debug
				done = true
			}
			marker = response.Marker
		}
		close(c)
	}()

	return c
}
