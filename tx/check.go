package tx

import (
	"fmt"

	"github.com/rubblelabs/ripple/data"
)

func NewCheckCreate(options ...func(data.Transaction) error) (*data.CheckCreate, error) {
	tx := &data.CheckCreate{
		TxBase: data.TxBase{
			TransactionType: data.CHECK_CREATE,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func NewCheckCash(options ...func(data.Transaction) error) (*data.CheckCash, error) {
	tx := &data.CheckCash{
		TxBase: data.TxBase{
			TransactionType: data.CHECK_CASH,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func NewCheckCancel(options ...func(data.Transaction) error) (*data.CheckCancel, error) {
	tx := &data.CheckCancel{
		TxBase: data.TxBase{
			TransactionType: data.CHECK_CANCEL,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func SetExpiration(rippleTime uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetCancelAfter()", tx)

		case *data.CheckCreate:
			tx.Expiration = &rippleTime

		}
		return nil
	}
}

func SetCheckID(id data.Hash256) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetCheckID()", tx)

		case *data.CheckCash:
			tx.CheckID = id

		case *data.CheckCancel:
			tx.CheckID = id
		}
		return nil

	}
}
