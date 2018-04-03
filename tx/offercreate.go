package tx

import (
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func NewOfferCreate(options ...func(data.Transaction) error) (*data.OfferCreate, error) {
	tx := &data.OfferCreate{
		TxBase: data.TxBase{
			TransactionType: data.OFFER_CREATE,
		},
	}
	err := Prepare(tx, options...)
	return tx, err
}

func SetTakerPays(amt interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.OfferCreate)
		if !ok {
			return errors.Errorf("Expected OfferCreate transaction, got %s", tx.GetBase().TransactionType)
		}

		var amount *data.Amount
		var err error

		switch amt := amt.(type) {
		case string:
			amount, err = data.NewAmount(amt)
			if err != nil {
				return errors.Wrapf(err, "Invalid amount: %s", amt)
			}
		case data.Amount:
			amount = &amt
		case *data.Amount:
			amount = amt
		}
		t.TakerPays = *amount
		return nil
	}
}

func SetTakerGets(amt interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.OfferCreate)
		if !ok {
			return errors.Errorf("Expected OfferCreate transaction, got %s", tx.GetBase().TransactionType)
		}

		var amount *data.Amount
		var err error

		switch amt := amt.(type) {
		case string:
			amount, err = data.NewAmount(amt)
			if err != nil {
				return errors.Wrapf(err, "Invalid amount: %s", amt)
			}
		case data.Amount:
			amount = &amt
		case *data.Amount:
			amount = amt
		}
		t.TakerGets = *amount
		return nil
	}
}
