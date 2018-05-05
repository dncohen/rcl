package rippledata

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

type AccountExchangesResponse struct {
	Response
	Count     int    `json:"count"`
	Marker    string `json:"marker"`
	Exchanges []Exchange
}

// https://ripple.com/build/data-api-v2/#exchange-objects
type Exchange struct {
	TransactionResponse

	BaseAmount      data.NonNativeValue `json:"base_amount"`
	BaseCurrency    string              `json:"base_currency"`
	BaseIssuer      string              `json:"base_issuer"`
	CounterAmount   data.NonNativeValue `json:"counter_amount"`
	CounterCurrency string              `json:"counter_currency"`
	CounterIssuer   string              `json:"counter_issuer"`
	Rate            data.NonNativeValue `json:"rate"`

	Buyer         data.Account `json:"buyer"`
	Seller        data.Account `json:"seller"`
	Provider      data.Account `json:"provider"`
	Taker         data.Account `json:"taker"`
	OfferSequence uint32       `json:"offer_sequence"`
}

func (this Exchange) GetBaseAmount() *data.Amount {
	str := fmt.Sprintf("%s/%s", this.BaseAmount, this.BaseCurrency)
	if this.BaseCurrency != "XRP" {
		str = fmt.Sprintf("%s/%s", str, this.BaseIssuer)
	}
	amount, err := data.NewAmount(str)
	if err != nil {
		log.Panic(errors.Wrapf(err, "GetBaseAmount(): bad amount %s", str))
	}
	return amount
}

func (this Exchange) GetCounterAmount() *data.Amount {
	str := fmt.Sprintf("%s/%s", this.CounterAmount, this.CounterCurrency)
	if this.CounterCurrency != "XRP" {
		str = fmt.Sprintf("%s/%s", str, this.CounterIssuer)
	}
	amount, err := data.NewAmount(str)
	if err != nil {
		log.Panic(errors.Wrapf(err, "GetCounterAmount(): bad amount %s", str))
	}
	return amount
}

func (this Client) GetAccountExchanges(account data.Account, marker string) (response *AccountExchangesResponse, err error) {
	response = &AccountExchangesResponse{}
	endpoint := this.Endpoint("accounts", account.String(), "exchanges")
	values := endpoint.Query()
	if marker != "" {
		values.Set("marker", marker)
	}
	err = this.Get(response, endpoint, &values)

	//q.Q(response, err)
	return
}

func (this Client) GetAccountExchangesAsync(account data.Account) chan Exchange {
	c := make(chan Exchange) // TODO buffer

	go func() {
		done := false
		marker := ""
		for !done {
			//log.Printf("GetAccountExchangesAsync: Requesting exchanges for %s with marker %s", account, marker) // debug
			response, err := this.GetAccountExchanges(account, marker)
			if err != nil {
				log.Panic(err)
			}
			for _, event := range response.Exchanges {
				c <- event
			}
			if response.Marker == "" {
				log.Printf("GetAccountExchangesAsync: No more exchanges for %s", account) // debug
				done = true
			}
			marker = response.Marker
		}
		close(c)
	}()

	return c
}
