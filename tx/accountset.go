package tx

import (
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func NewAccountSet(options ...func(data.Transaction) error) (*data.AccountSet, error) {
	tx := &data.AccountSet{TxBase: data.TxBase{TransactionType: data.ACCOUNT_SET}}
	err := Prepare(tx, options...)
	return tx, err
}

func SetAccountFlag(flag uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.AccountSet)
		if !ok {
			return errors.New("SetAccountFlag expected AccountSet transaction.")
		}
		t.SetFlag = &flag
		return nil
	}
}

func SetDomain(domain *string) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.AccountSet)
		if !ok {
			return errors.Errorf("Expected AccountSet transaction, got %s", tx.GetBase().TransactionType)
		}

		if domain == nil {
			// No change to domain.  Note that "" is used to unset the domain.
		} else {
			bytes := data.VariableLength(*domain)
			t.Domain = &bytes
		}

		return nil
	}
}

// SetMessageKey populates the MessageKey field of an AccountSet transaction.
//
// See https://xrpl.org/accountset.html
//
func SetMessageKey(messageKey interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		t, ok := tx.(*data.AccountSet)
		if !ok {
			return errors.Errorf("Expected AccountSet transaction, got %s", tx.GetBase().TransactionType)
		}

		if messageKey == nil {
			// No change to domain.  Note that "" is used to unset the message key.
			return nil
		}

		var val data.VariableLength

		switch in := messageKey.(type) {
		// Note only the []byte path here is known to work.  MessageKey is
		// documented as a string, but transaction will fail to validate
		// unless it is set just right.  And how to do that is not made clear.

		case []byte:
			// this code path has worked
			if in == nil {
				return nil
			}
			val = in

		case *string:
			if in == nil {
				return nil
			}
			val = []byte(*in)

		case string:
			val = []byte(in)

		}

		t.MessageKey = &val

		return nil
	}
}
