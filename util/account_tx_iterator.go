package util

import (
	"fmt"
	"log"

	"github.com/dncohen/rcl/rpc"
	"github.com/pkg/errors"
	"github.com/y0ssar1an/q"
)

type AccountTxIterator struct {
	client         rpc.Client
	ledgerIndexMin uint32
	ledgerIndexMax uint32
	address        string
	current        *rpc.AccountTxResult
	currentIndex   int
	forward        bool
	err            error
	more           bool
	advance        bool

	// prev is for a sanity check (that we are returning always in the intended order)
	prev *rpc.TxMeta
}

// Sets the error encountered by iterator.  On the *first* error is remembered.
func (iter *AccountTxIterator) setError(err error) {
	if iter.err == nil {
		iter.err = err
	}
}

func NewAccountTxIterator(client rpc.Client, address string, forward bool, min, max uint32) *AccountTxIterator {
	return &AccountTxIterator{
		client:         client,
		address:        address,
		forward:        forward,
		ledgerIndexMin: min,
		ledgerIndexMax: max,
	}
}

func (iter *AccountTxIterator) Close() {
	iter.client.Close()
}

func (iter *AccountTxIterator) Next() bool {

	// Perhaps we've already queried, and still have results to return.
	if iter.current != nil && iter.currentIndex < len(iter.current.Transactions)-1 {
		iter.currentIndex++
		iter.more = true
	} else if iter.current != nil && iter.current.Marker == nil && !iter.advance {
		// If last query had no marker, no more results.
		iter.more = false
	} else {

		// Don't advance again, until Advance() is called again.
		iter.advance = false

		// If not, either make a first query, or a query with a marker.
		params := rpc.AccountTxParams{
			Account:          iter.address,
			Ledger_index_min: iter.ledgerIndexMin,
			Ledger_index_max: iter.ledgerIndexMax,
			//Limit:            1,
			Forward: iter.forward,
		}
		if iter.current != nil {
			params.Marker = iter.current.Marker
		}

		// Get more tx from rippled
		//_, err := iter.client.Do("account_tx", params, &iter.current)
		response, err := iter.client.Request("account_tx", params)
		if err != nil {
			log.Printf("account_tx %s: %s", iter.address, err)
			iter.setError(err)
		} else {

			// Replace iter.current (if we unmarshal over the old one, Marker will not be overwritten)
			newCurrent := &rpc.AccountTxResult{}

			err = response.UnmarshalResult(newCurrent)
			if err != nil {
				log.Printf("account_tx %s: %s", iter.address, err)
				iter.setError(err)
			} else {

				iter.current = newCurrent
				iter.currentIndex = 0
				iter.more = len(iter.current.Transactions) > 0
			}
		}
	}

	return iter.more
}

func (iter *AccountTxIterator) Get() (rpc.TxMeta, error) {
	txMeta := iter.current.Transactions[iter.currentIndex]

	if iter.prev != nil {
		// Sanity check.

		prevLedger := iter.prev.Tx.Ledger_index
		prevIndex := iter.prev.Meta.TransactionIndex

		ledger := txMeta.Tx.Ledger_index
		index := txMeta.Meta.TransactionIndex

		prevBefore := (prevLedger < ledger) || (prevLedger == ledger && prevIndex < index)
		if (prevBefore && !iter.forward) || (!prevBefore && iter.forward) {
			q.Q(iter.prev)
			q.Q(txMeta)

			// Should never be here.  If this occurs, we need code to sort transactions.
			panic(fmt.Sprintf("account_tx iterator: %s and %s in reverse order!", iter.prev.Tx.Hash, txMeta.Tx.Hash))
		}

	}
	iter.prev = &txMeta // Remember, for next sanity check.

	return txMeta, iter.err
}

func (iter *AccountTxIterator) Error() error {
	return iter.err
}

func (iter *AccountTxIterator) Advance(index uint32) error {

	min := iter.ledgerIndexMin
	max := iter.ledgerIndexMax

	// Checks
	if iter.more {
		return errors.New("Cannot advance account_tx_iterator with more to Get()")
	}

	if iter.forward {
		min = iter.ledgerIndexMax + 1
		max = index
	} else {
		max = iter.ledgerIndexMin - 1
		min = index
	}

	if min > max {
		return errors.Errorf("Invalid ledger range %d > %d", min, max)
	}

	iter.ledgerIndexMax = max
	iter.ledgerIndexMin = min
	iter.advance = true

	return nil
}

func (iter *AccountTxIterator) Bounds() (uint32, uint32) {
	return iter.ledgerIndexMin, iter.ledgerIndexMax
}
