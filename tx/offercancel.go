package tx

import (
	"fmt"

	"github.com/rubblelabs/ripple/data"
)

func NewOfferCancel(options ...func(data.Transaction) error) (*data.OfferCancel, error) {
	tx := &data.OfferCancel{
		TxBase: data.TxBase{
			TransactionType: data.OFFER_CANCEL,
		},
	}
	err := Prepare(tx, options...)
	return tx, err
}

func SetOfferSequence(offerSeq uint32) func(data.Transaction) error {
	return func(tx data.Transaction) error {

		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetOfferSequence", tx)

		case *data.OfferCancel:
			tx.OfferSequence = offerSeq
		case *data.EscrowFinish:
			tx.OfferSequence = offerSeq
		case *data.EscrowCancel:
			tx.OfferSequence = offerSeq
		}
		return nil
	}
}
