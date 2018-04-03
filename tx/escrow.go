package tx

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func NewEscrowCreate(options ...func(data.Transaction) error) (*data.EscrowCreate, error) {
	tx := &data.EscrowCreate{
		TxBase: data.TxBase{
			TransactionType: data.ESCROW_CREATE,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func NewEscrowFinish(options ...func(data.Transaction) error) (*data.EscrowFinish, error) {
	tx := &data.EscrowFinish{
		TxBase: data.TxBase{
			TransactionType: data.ESCROW_FINISH,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func NewEscrowCancel(options ...func(data.Transaction) error) (*data.EscrowCancel, error) {
	tx := &data.EscrowCancel{
		TxBase: data.TxBase{
			TransactionType: data.ESCROW_CANCEL,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func SetFinishAfter(rippleTime uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetCancelAfter()", tx)

		case *data.EscrowCreate:
			tx.FinishAfter = &rippleTime

		}
		return nil
	}
}

func SetOwner(address interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		var account *data.Account
		var err error

		switch address := address.(type) {
		default:
			return fmt.Errorf("Unexpected address type %T in SetOwner()", address)

		case string:
			account, err = data.NewAccountFromAddress(address)
			if err != nil {
				return errors.Wrapf(err, "Bad address %s", address)
			}

		case data.Account:
			account = &address
		case *data.Account:
			account = address

		}

		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetOwner()", tx)

		case *data.EscrowFinish:
			tx.Owner = *account
		case *data.EscrowCancel:
			tx.Owner = *account

		}

		return nil
	}
}
