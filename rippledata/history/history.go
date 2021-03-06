package history

import (
	"container/heap"

	"github.com/dncohen/rcl/rippledata"
	"github.com/rubblelabs/ripple/data"
	"golang.org/x/sync/errgroup"
)

// history package iterates through the activity of multiple RCL
// accounts in chronological order.

// Ripple data APIs allow us to iterate through a single account's
// history.  We support multiple accounts, and we want to iterate
// through their history in chronological order.  The `pendingQueue`
// helps us do this, by arranging the oldest transactions for
// several accounts in chronological order.  We're collecing and
// ordering transactions in pendingQueue.  It is FIFO (oldest first)
// regardless of whether our cost basis is calculated FIFO or LIFO.

// AccountTx struct groups account data with transaction data
type AccountTx struct {
	// Embed Transaction so this can be used in transactionHeap
	rippledata.Transaction
	Nick    string
	Account *data.Account
}

type BalanceChangeIterator struct {
	client rippledata.Client

	// These maps are keyed by account nickname
	accounts       map[string]*data.Account // Accounts, by nickname
	AccountData    map[string]rippledata.AccountData
	balanceChanges map[string]chan rippledata.BalanceChangeDescriptor

	// Not yet processed events, ordered chronologically
	pendingQueue heap.Interface

	// The same transaction may generate multiple events.  We collect all such events in duplicateBuffer
	//duplicateBuffer []*AccountTx
}

func NewBalanceChangeIterator(client rippledata.Client, accounts map[string]*data.Account) *BalanceChangeIterator {

	pendingQueue := make(rippledata.TransactionFIFO, 0, len(accounts)*2)
	this := BalanceChangeIterator{
		client:       client,
		pendingQueue: &pendingQueue,
		//duplicateBuffer: make([]*AccountTx, 0, 2),

		accounts:       accounts,
		AccountData:    make(map[string]rippledata.AccountData, len(accounts)),
		balanceChanges: make(map[string]chan rippledata.BalanceChangeDescriptor, len(accounts)),
	}
	return &this
}

// Initializing a balance change iterator is time consuming as it
// queries rippledata for account information.
func (this *BalanceChangeIterator) Init() error {
	heap.Init(this.pendingQueue) // transactions to be processed, oldest first.

	err := this.getAccountData()
	if err != nil {
		return err
	}

	// Initialize channels for each account.
	for nick, account := range this.accounts {
		this.balanceChanges[nick] = this.client.GetBalanceChangesAsync(*account)
	}

	// Initialize pending queue with the first transactions for each account.
	for nick, _ := range this.accounts {
		this.queueBalanceChange(nick)
	}

	return err
}

func (this *BalanceChangeIterator) getAccountData() error {
	// errgroup helps us do things concurrently
	var g errgroup.Group

	for nick, account := range this.accounts {
		account := account // scope!
		nick := nick
		g.Go(func() error {

			response, err := this.client.AccountData(*account)
			if err != nil {
				return err
			}
			//log.Printf("Got account data for %s = %s", nick, account) // debug
			this.AccountData[nick] = response.AccountData
			return nil
		})
	}
	return g.Wait()
}

func (this *BalanceChangeIterator) queueBalanceChange(nick string) {
	event, ok := <-this.balanceChanges[nick]
	if ok {
		heap.Push(this.pendingQueue, &AccountTx{
			Transaction: event,
			Nick:        nick,
			Account:     this.accounts[nick],
		})
	}
}

// Next returns all ripple data events associated with the next
// pending balance change transaction.
func (this *BalanceChangeIterator) Next() []*AccountTx {

	if this.pendingQueue.Len() == 0 {
		return nil
	}

	// We're going to return all events generated by the next
	// transaction.  Could be more than one event.
	events := make([]*AccountTx, 0, 2)
	done := false

	for !done {
		event := heap.Pop(this.pendingQueue).(*AccountTx)
		// Every time we pop, queue that account's next event
		this.queueBalanceChange(event.Nick)

		if len(events) == 0 || event.GetHash() == events[0].GetHash() {
			events = append(events, event)
		} else {
			// The most recently popped event should be returned next time.
			heap.Push(this.pendingQueue, event)
			done = true
		}
		if this.pendingQueue.Len() == 0 {
			// No more events
			done = true
		}
	}

	return events
}
