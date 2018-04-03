package tx

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func NewPayment(options ...func(data.Transaction) error) (*data.Payment, error) {
	tx := &data.Payment{
		TxBase: data.TxBase{
			TransactionType: data.PAYMENT,
		},
	}
	err := Prepare(tx, options...)

	return tx, err
}

func SetInvoiceID(id interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.Payment)
		if !ok {
			return errors.Errorf("Expected Payment transaction, got %s", tx.GetBase().TransactionType)
		}
		var err error
		switch v := id.(type) {
		case string:
			t.InvoiceID, err = data.NewHash256(v)
		case data.Hash256:
			t.InvoiceID = &v
		case *data.Hash256:
			t.InvoiceID = v
		default:
			return fmt.Errorf("SetInvoiceID: Wrong type %+v", v)
		}

		return err
	}
}

func SetSendMax(amt interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		var amount *data.Amount
		var err error
		switch amt := amt.(type) {
		case string:
			amount, err = data.NewAmount(amt)
			if err != nil {
				return errors.Wrapf(err, "Invalid amount: %s", amt)
			}
		case *data.Amount:
			amount = amt
		default:
			return fmt.Errorf("SetSendMax: wrong type %+v", amt)
		}

		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetSendMax()", tx)
		case *data.Payment:
			tx.SendMax = amount
		case *data.CheckCreate:
			tx.SendMax = *amount
		}
		return nil
	}
}

func SetDeliverMin(amt interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		var amount *data.Amount
		var err error
		switch amt := amt.(type) {
		case string:
			amount, err = data.NewAmount(amt)
			if err != nil {
				return errors.Wrapf(err, "Invalid amount: %s", amt)
			}
		case *data.Amount:
			amount = amt
		default:
			return fmt.Errorf("SetDeliverMin: wrong type %+v", amt)
		}

		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetDeliverMin()", tx)
		case *data.Payment:
			tx.DeliverMin = amount
		}
		return nil
	}
}
