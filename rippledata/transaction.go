package rippledata

import (
	"encoding/json"
	"log"
	"regexp"
	"time"

	"github.com/rubblelabs/ripple/data"
)

type GetTransactionResponse struct {
	Response
	Transaction struct {
		LedgerIndex uint32       `json:"ledger_index"`
		Date        time.Time    `json:"date"`
		Hash        data.Hash256 `json:"hash"`

		// Tx field is data.TransactionWithMetaData (instead of
		// data.TxBase) because TransactionWithMetaData knows how to
		// unmarshal many types of transaction from JSON.
		Tx   data.TransactionWithMetaData `json:"tx"`   // json from Data API actually doesn't have metadata :(
		Meta data.MetaData                `json:"meta"` // see postop(), we copy this into Tx

	} `json:"transaction"`
}

var fixSignerEntry = regexp.MustCompile(`{\s*"SignerEntry":\s*({[^}]*})\s*}`)

// Data API structures differ in quirky ways from rippled data
// structures.  This kludge unmangles some of the damage.
func (this *GetTransactionResponse) triage(in json.RawMessage) json.RawMessage {
	out := fixSignerEntry.ReplaceAllString(string(in), `$1`)
	return json.RawMessage(out)
}

// Like triage, this kluge reconciles differences between ripple data
// API structures vs rippled data structures.
func (this *GetTransactionResponse) postop() {
	// These differences, Data API vs rippled, that I've noticed.  There are likely many more.
	this.Transaction.Tx.GetBase().Hash = this.Transaction.Hash
	this.Transaction.Tx.MetaData = this.Transaction.Meta
}

func (this Client) Transaction(hash data.Hash256) (*GetTransactionResponse, error) {
	endpoint := this.Endpoint("transactions", hash.String())
	response := &GetTransactionResponse{}
	err := this.Get(response, endpoint, nil)
	return response, err
}

// Common fields in data responses that identify transactions.  I.e. payments, exchanges and balance changes
type TransactionResponse struct { // TODO(dnc): TransactionResponse is a bad name for what this is (does not embed Response)
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

// Helper type for TransactionFIFO and TransactionLIFO
type transactionHeap []Transaction

func newTransactionQueue() *transactionHeap {
	tq := make(transactionHeap, 0, 1)
	return &tq
}

func (this transactionHeap) Len() int { return len(this) }

func (this transactionHeap) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

// Push and Pop use pointer receivers because they modify slice's length
func (this *transactionHeap) Push(x interface{}) {
	*this = append(*this, x.(Transaction))
}

func (this *transactionHeap) Pop() interface{} {
	old := *this
	n := len(old)
	item := old[n-1]
	*this = old[0 : n-1]
	return item
}

type TransactionLIFO transactionHeap

func NewTransactionLIFO() *TransactionLIFO {
	return (*TransactionLIFO)(newTransactionQueue())
}

// Less determines whether heap is LIFO or FIFO
func (this TransactionLIFO) Less(i, j int) bool {
	if this[i].GetLedgerIndex() > this[j].GetLedgerIndex() {
		return true
	} else if this[i].GetLedgerIndex() == this[j].GetLedgerIndex() {
		// Unfortunately tx_index is not returned from all APIs.  It is returned from balance_changes.
		return this[i].GetTransactionIndex() > this[j].GetTransactionIndex()
	}
	return false
}

func (this TransactionLIFO) Len() int {
	return transactionHeap(this).Len()
}

func (this TransactionLIFO) Swap(i, j int) {
	transactionHeap(this).Swap(i, j)
}

// Push and Pop use pointer receivers because they modify slice's length
func (this *TransactionLIFO) Push(x interface{}) {
	(*transactionHeap)(this).Push(x)
}
func (this *TransactionLIFO) Pop() interface{} {
	return (*transactionHeap)(this).Pop()
}

type TransactionFIFO transactionHeap

func NewTransactionFIFO() *TransactionFIFO {
	return (*TransactionFIFO)(newTransactionQueue())
}

// Less determines whether heap is LIFO or FIFO.  Pay attention to less than.
func (this TransactionFIFO) Less(i, j int) bool {
	if this[i].GetLedgerIndex() < this[j].GetLedgerIndex() {
		return true
	} else if this[i].GetLedgerIndex() == this[j].GetLedgerIndex() {
		// Unfortunately tx_index is not returned from all APIs.  It is returned from balance_changes.
		return this[i].GetTransactionIndex() < this[j].GetTransactionIndex()
	}
	return false
}

func (this TransactionFIFO) Len() int {
	return transactionHeap(this).Len()
}

func (this TransactionFIFO) Swap(i, j int) {
	transactionHeap(this).Swap(i, j)
}

// Push and Pop use pointer receivers because they modify slice's length
func (this *TransactionFIFO) Push(x interface{}) {
	(*transactionHeap)(this).Push(x)
}
func (this *TransactionFIFO) Pop() interface{} {
	return (*transactionHeap)(this).Pop()
}
