package rippledata

import (
	"log"
	"time"

	"github.com/rubblelabs/ripple/data"
)

// Common fields in data responses that identify transactions.  I.e. payments, exchanges and balance changes

type TransactionResponse struct {
	TxHash       data.Hash256 `json:"tx_hash"`
	LedgerIndex  uint32       `json:"ledger_index"`
	ExecutedTime time.Time    `json:"executed_time"`

	// Balance change has tx_index, but payment and exchanges do not :(
}

type Transaction interface {
	GetHash() data.Hash256
	GetLedgerIndex() uint32
	GetTransactionIndex() uint32 // balance_changes returns this, but exchanges and payments do not!
	GetExecutedTime() time.Time
}

func (this TransactionResponse) GetHash() data.Hash256 {
	return this.TxHash
}
func (this TransactionResponse) GetLedgerIndex() uint32 {
	return this.LedgerIndex
}
func (this TransactionResponse) GetExecutedTime() time.Time {
	return this.ExecutedTime
}
func (this TransactionResponse) GetTransactionIndex() uint32 {
	log.Println("Unknown transaction index:", this.GetHash()) // debug
	return 0
}

type TransactionQueue []Transaction

func NewTransactionQueue() *TransactionQueue {
	tq := make(TransactionQueue, 0, 1)
	return &tq
}

func (this TransactionQueue) Len() int { return len(this) }

func (this TransactionQueue) Less(i, j int) bool {
	if this[i].GetLedgerIndex() < this[j].GetLedgerIndex() {
		return true
	} else if this[i].GetLedgerIndex() == this[j].GetLedgerIndex() {
		// Unfortunately tx_index is not returned from all APIs.  It is returned from balance_changes.
		return this[i].GetTransactionIndex() < this[j].GetTransactionIndex()
	}
	return false
}

func (this TransactionQueue) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

// Push and Pop use pointer receivers because they modify slice's length
func (this *TransactionQueue) Push(x interface{}) {
	*this = append(*this, x.(Transaction))
}

func (this *TransactionQueue) Pop() interface{} {
	old := *this
	n := len(old)
	item := old[n-1]
	*this = old[0 : n-1]
	return item
}
