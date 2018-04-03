package tx

import (
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func NewPaymentChannelCreate(options ...func(data.Transaction) error) (*data.PaymentChannelCreate, error) {
	tx := &data.PaymentChannelCreate{
		TxBase: data.TxBase{
			TransactionType: data.PAYCHAN_CREATE,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

// Deprecated, use SetDestination()
func SetChannelDestinationXXX(account data.Account) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelCreate)
		if !ok {
			return errors.Errorf("Expected PaymentChannelCreate transaction, got %s", tx.GetBase().TransactionType)
		}

		t.Destination = account
		return nil
	}
}

// Deprecated, use SetDestinationTag()
func SetChannelDestinationTagXXX(tag *uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelCreate)
		if !ok {
			return errors.Errorf("Expected PaymentChannelCreate transaction, got %s", tx.GetBase().TransactionType)
		}

		t.DestinationTag = tag
		return nil
	}
}

// Deprecated, use SetAmount()
func SetChannelAmountXXX(amountStr string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelCreate)
		if !ok {
			return errors.Errorf("Expected PaymentChannelCreate transaction, got %s", tx.GetBase().TransactionType)
		}
		amount, err := data.NewAmount(amountStr)
		if err != nil || !amount.IsNative() { // channels support only XRP.
			return errors.Wrapf(err, "Invalid amount: %s", amountStr)
		}
		t.Amount = *amount
		return nil
	}
}

func SetChannelPublicKey(key data.PublicKey) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelCreate)
		if !ok {
			return errors.Errorf("Expected PaymentChannelCreate transaction, got %s", tx.GetBase().TransactionType)
		}
		t.PublicKey = key
		return nil
	}
}

func SetChannelSettleDelay(delay uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelCreate)
		if !ok {
			return errors.Errorf("Expected PaymentChannelCreate transaction, got %s", tx.GetBase().TransactionType)
		}
		t.SettleDelay = delay
		return nil
	}
}

func NewPaymentChannelFund(options ...func(data.Transaction) error) (*data.PaymentChannelFund, error) {
	tx := &data.PaymentChannelFund{
		TxBase: data.TxBase{
			TransactionType: data.PAYCHAN_FUND,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}
func NewPaymentChannelClaim(options ...func(data.Transaction) error) (*data.PaymentChannelClaim, error) {
	tx := &data.PaymentChannelClaim{
		TxBase: data.TxBase{
			TransactionType: data.PAYCHAN_CLAIM,
		},
	}
	err := Prepare(tx, options...)

	return tx, err

}

func SetClaimChannel(id data.Hash256) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelClaim)
		if !ok {
			return errors.Errorf("Expected PaymentChannelClaim transaction, got %s", tx.GetBase().TransactionType)
		}
		t.Channel = id
		return nil
	}
}

func SetClaimAmount(amountStr string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelClaim)
		if !ok {
			return errors.Errorf("Expected PaymentChannelClaim transaction, got %s", tx.GetBase().TransactionType)
		}
		amount, err := data.NewAmount(amountStr)
		if err != nil || !amount.IsNative() { // channels support only XRP.
			return errors.Wrapf(err, "Invalid amount: %s", amountStr)
		}
		t.Amount = amount
		return nil
	}
}

func SetClaimBalance(amountStr string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelClaim)
		if !ok {
			return errors.Errorf("Expected PaymentChannelClaim transaction, got %s", tx.GetBase().TransactionType)
		}
		amount, err := data.NewAmount(amountStr)
		if err != nil || !amount.IsNative() { // channels support only XRP.
			return errors.Wrapf(err, "Invalid amount: %s", amountStr)
		}
		t.Balance = amount
		return nil
	}
}

func SetClaimClose(value bool) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.PaymentChannelClaim)
		if !ok {
			return errors.Errorf("Expected PaymentChannelClaim transaction, got %s", tx.GetBase().TransactionType)
		}
		return Flags(t, data.TxClose, value)
	}
}
