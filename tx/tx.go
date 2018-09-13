package tx

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

// Helpers to create rubblelabs data.Transaction

func Prepare(tx data.Transaction, options ...func(data.Transaction) error) error {
	for _, option := range options {
		err := option(tx)
		if err != nil {
			return err
		}
	}

	// Validate fields required for all transactions.
	base := tx.GetBase()
	if base.TransactionType == 0 {
		// This test is not sufficient because data.PAYMENT == 0.
		// TODO ensure here that transaction is a payment.
		//return errors.New("Transaction requires type.")
	}
	if base.Sequence == 0 {
		return errors.New("Transaction requires an account sequence number.")
	}

	return nil
}

func SetAddress(address interface{}) func(data.Transaction) error {
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
		tx.GetBase().Account = *account
		return nil
	}
}

func SetSequence(seq uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {

		tx.GetBase().Sequence = seq
		return nil
	}
}

func SetLastLedgerSequence(seq uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {

		tx.GetBase().LastLedgerSequence = &seq
		return nil
	}
}

func SetFee(f int) func(data.Transaction) error {
	return func(tx data.Transaction) error {

		fee, err := data.NewNativeValue(int64(f))
		if err != nil {
			return errors.Wrapf(err, "Bad fee %s", fee)
		}
		tx.GetBase().Fee = *fee
		return nil
	}
}

func Flags(tx data.Transaction, flag data.TransactionFlag, onOrOff bool) error {
	base := tx.GetBase()
	if base.Flags == nil {
		var f data.TransactionFlag
		base.Flags = &f
	}
	if onOrOff {
		*base.Flags = *base.Flags | flag
	} else {
		*base.Flags = *base.Flags &^ flag
	}
	return nil
}

func SetFlags(flag data.TransactionFlag) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		return Flags(tx, flag, true)
	}
}

func SetCanonicalSig(value bool) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		return Flags(tx, data.TxCanonicalSignature, value)
	}
}

func AddMemo(value interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		base := tx.GetBase()

		var memoType, memoData, memoFormat data.VariableLength

		switch v := value.(type) {
		case []byte:
			if len(v) == 0 {
				return nil // No memo
			} else {
				memoData = v
			}
		case string:
			memoData = []byte(v)
		case *string:
			if v == nil {
				return nil // nil pointer means no memo
			} else {
				memoData = []byte(*v)
			}
		case data.Hash256:
			memoData = v.Bytes()
		case *data.Hash256:
			if v == nil {
				return nil // nil pointer means no memo
			} else {
				memoData = v.Bytes()
			}
		default:
			return fmt.Errorf("Unexpected type %T (\"%+v\") passed to AddMemo.", v, v)
		}

		// This seems redundant to define this struct here, but I haven't
		// found a better way to construct the rubblelabs memo structure!
		memo := struct {
			// This has to exactly match the data.Memo definition!
			MemoType   data.VariableLength
			MemoData   data.VariableLength
			MemoFormat data.VariableLength
		}{
			MemoType:   memoType,
			MemoData:   memoData,
			MemoFormat: memoFormat,
		}
		base.Memos = append(base.Memos, data.Memo{Memo: memo})

		return nil
	}
}

// Sets the destination account for transaction types which expect a destination.
func SetDestination(account data.Account) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetDestination()", tx)
		case *data.PaymentChannelCreate:
			tx.Destination = account
		case *data.Payment:
			tx.Destination = account
		case *data.EscrowCreate:
			tx.Destination = account
		case *data.CheckCreate:
			tx.Destination = account
		}
		return nil
	}
}

func SetDestinationTag(tag *uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetDestinationTag()", tx)
		case *data.PaymentChannelCreate:
			tx.DestinationTag = tag
		case *data.Payment:
			tx.DestinationTag = tag
		case *data.EscrowCreate:
			tx.DestinationTag = tag
		case *data.CheckCreate:
			tx.DestinationTag = tag
		}
		return nil
	}
}

func SetSourceTag(tag *uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		tx.GetBase().SourceTag = tag
		return nil
	}
}

func SetAmount(amt interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {

		var amount *data.Amount
		var err error

		switch amt := amt.(type) {
		default:
			return fmt.Errorf("SetAmount: unexpected amount type %T", amt)
		case string:
			amount, err = data.NewAmount(amt)
			if err != nil {
				return errors.Wrapf(err, "Invalid amount: %s", amt)
			}
		case *data.Amount:
			amount = amt
		case data.Amount:
			amount = &amt
		}

		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetAmount()", tx)

		case *data.Payment:
			tx.Amount = *amount

		case *data.PaymentChannelCreate:
			if !amount.IsNative() { // support only XRP.
				return errors.Wrapf(err, "Invalid amount (non-XRP): %s", amt)
			}
			tx.Amount = *amount

		case *data.EscrowCreate:
			if !amount.IsNative() { // support only XRP.
				return errors.Wrapf(err, "Invalid amount (non-XRP): %s", amt)
			}
			tx.Amount = *amount

		case *data.CheckCash:
			tx.Amount = amount

		}
		return nil
	}
}

func SetCancelAfter(rippleTime uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetCancelAfter()", tx)

		case *data.PaymentChannelCreate:
			tx.CancelAfter = &rippleTime
		case *data.EscrowCreate:
			tx.CancelAfter = &rippleTime

		}
		return nil
	}
}
