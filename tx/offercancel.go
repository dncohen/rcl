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

func SetOfferSequence(offerSeq interface{}) func(data.Transaction) error {
	return func(tx data.Transaction) error {
		var os uint32

		// We accept uint32 or *uint32, ignore if nil.
		switch offerSeq := offerSeq.(type) {
		default:
			return fmt.Errorf("Unexpected offer sequence type %T in SetOfferSequence", offerSeq)
		case uint32:
			os = offerSeq
		case *uint32:
			if offerSeq != nil {
				os = *offerSeq
			} else {
				return nil
			}
		}

		switch tx := tx.(type) {
		default:
			return fmt.Errorf("Unexpected transaction type %T in SetOfferSequence", tx)

		case *data.OfferCancel:
			tx.OfferSequence = os
		case *data.OfferCreate:
			tx.OfferSequence = &os // optional in OfferCreate
		case *data.EscrowFinish:
			tx.OfferSequence = os
		case *data.EscrowCancel:
			tx.OfferSequence = os
		}
		return nil
	}
}
