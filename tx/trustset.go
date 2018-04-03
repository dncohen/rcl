package tx

import (
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func NewTrustSet(options ...func(data.Transaction) error) (*data.TrustSet, error) {
	tx := &data.TrustSet{
		TxBase: data.TxBase{
			TransactionType: data.TRUST_SET,
		},
	}
	err := Prepare(tx, options...)
	// TODO validate complete limit amount specified
	return tx, err
}

func SetTrustAuthorize(authorized bool) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		base := tx.GetBase()
		_, ok := tx.(*data.TrustSet)
		if !ok {
			return errors.Errorf("Expected TrustSet transaction, got %s", base.TransactionType)
		}

		return Flags(tx, data.TxSetAuth, true)
	}
}

func SetLimitCurrency(currency string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.TrustSet)
		if !ok {
			return errors.Errorf("Expected TrustSet transaction, got %s", tx.GetBase().TransactionType)
		}

		var err error
		t.LimitAmount.Currency, err = data.NewCurrency(currency)

		return err
	}
}

func SetLimitIssuer(address string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.TrustSet)
		if !ok {
			return errors.Errorf("Expected TrustSet transaction, got %s", tx.GetBase().TransactionType)
		}

		issuer, err := data.NewAccountFromAddress(address)
		if err != nil {
			return err
		}
		t.LimitAmount.Issuer = *issuer
		return nil
	}
}

func SetLimitValue(s string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.TrustSet)
		if !ok {
			return errors.Errorf("Expected TrustSet transaction, got %s", tx.GetBase().TransactionType)
		}

		value, err := data.NewValue(s, false)
		if err != nil {
			return err
		}
		t.LimitAmount.Value = value

		return err
	}
}

func SetLimitAmount(a data.Amount) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.TrustSet)
		if !ok {
			return errors.Errorf("Expected TrustSet transaction, got %s", tx.GetBase().TransactionType)
		}
		t.LimitAmount = a
		return nil
	}
}
