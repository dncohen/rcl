package util

import (
	"container/heap"
	"container/list"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/terminal"
	"github.com/rubblelabs/ripple/websockets"
	"github.com/y0ssar1an/q"
)

type Subscription struct {
	// Keep track of contiguous ledger history we have available.
	min, max uint32
	mutex    sync.RWMutex

	// Connection to the rippled
	url       string
	result    *websockets.SubscribeResult
	connected chan bool

	// Make "Remote" public so that calls can made to it directly.
	Remote *websockets.Remote

	sequenceListeners *ledgerSequenceWaitHeap
	timeListeners     *ledgerTimeWaitHeap
	txListeners       *list.List
}

func NewSubscription(wss string) (*Subscription, error) {
	sub := &Subscription{
		url:               wss,
		sequenceListeners: &ledgerSequenceWaitHeap{},
		timeListeners:     &ledgerTimeWaitHeap{},
		connected:         make(chan bool), // A closed channel never blocks.
	}
	// Collection of listeners we will notify when ledger sequence has been validated.
	heap.Init(sub.sequenceListeners)
	heap.Init(sub.timeListeners)
	sub.txListeners = list.New()

	err := sub.connect()
	return sub, err

}

// Connect, or reconnect to rippled.
func (sub *Subscription) connect() error {
	var err error

	sub.Remote, err = websockets.NewRemote(sub.url)
	if err != nil {
		return errors.Wrapf(err, "Failed to connect to rippled websocket %s", sub.url)
	}

	// Order of booleans passed to Subscribe() is ledger, transactions,
	// transactionsProposed, server.  (Does server even work?)
	sub.result, err = sub.Remote.Subscribe(true, false, false, false)
	if err != nil {
		return errors.Wrapf(err, "Failed to subscribe to %s", sub.url)
	}

	return err
}

func (sub *Subscription) SubmitWait(t data.Transaction) (*websockets.TxResult, error) {
	lastLedger := t.GetBase().LastLedgerSequence
	if lastLedger == nil {
		return nil, fmt.Errorf("Cannot wait for %s transaction without LastLedgerSequence.", t.GetType())
	}

	hash := t.GetHash()
	if hash == nil {
		return nil, fmt.Errorf("Cannot submit %s transaction. Not signed?", t.GetType())
	}

	_, ledgerBeforeSubmit, err := sub.Ledgers()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get current ledger index.")
	}

	if ledgerBeforeSubmit >= *lastLedger {
		return nil, fmt.Errorf("Too late to submit %s transaction with LastLedgerSequence %d.  Ledger index %d has already passed.", t.GetType(), *lastLedger, ledgerBeforeSubmit)
	}

	tentative, err := sub.Remote.Submit(t)
	_ = tentative // TODO log tentative result?
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to submit %s transaction.", t.GetType())
	}

	log.Printf("Tentative result of %s transaction by %s, %s: %s %s \n", t.GetType(), t.GetBase().Account, *hash, tentative.EngineResult, tentative.EngineResultMessage)
	log.Printf("Waiting for final result of %s...\n", *hash)

	result := <-sub.AfterTx(*hash, ledgerBeforeSubmit, *lastLedger)

	// verbose
	//log.Printf("%s %s %s in ledger %d\n", result.MetaData.TransactionResult, result.Transaction.GetType(), *hash, result.LedgerSequence)
	// TODO is it necessary to have timeout or handle disconnects, etc here?
	return result, nil
}

// Helper to sign and submit a transaction.
// Returns a func with errgroup.Group compatible signature.
func (sub *Subscription) SignAndSubmitFunc(wallet *Wallet, t data.Transaction) func() error {
	return func() error {
		_, _, err := wallet.Sign(t)
		if err != nil {
			return errors.Wrapf(err, "Failed to sign %s transaction", t.GetType())
		}

		result, err := sub.SubmitWait(t)
		if err != nil {
			return errors.Wrapf(err, "Failed to submit %s transaction", t.GetType())
		}

		if !result.Validated {
			q.Q(result)
			return fmt.Errorf("%s transactions failed to validate.", t.GetType())
		}

		if !result.MetaData.TransactionResult.Success() {
			return fmt.Errorf("%s transaction failed: %s", t.GetType(), result.MetaData.TransactionResult)
		}
		return err
	}
}

// Modelled on time.After(), this function provides a convenient way
// to wait for a specific ledger instance to pass.
func (sub *Subscription) AfterSequence(until uint32) <-chan uint32 {

	listener := ledgerSequenceWait{
		c:     make(chan uint32),
		until: until,
	}
	heap.Push(sub.sequenceListeners, listener)
	return listener.c
}

// Modelled on time.After(), this function provides a convenient way
// to wait for a specific ledger instance to pass.
func (sub *Subscription) AfterTime(when interface{}) <-chan data.RippleTime {

	var until *data.RippleTime
	switch when := when.(type) {
	case data.RippleTime:
		until = &when
	case *data.RippleTime:
		until = when
	case uint32:
		until = data.NewRippleTime(when)
	case *uint32:
		until = data.NewRippleTime(*when)
	default:
		log.Panicf("Unexpected %T in subscription.AfterTime()", when)
	}
	listener := ledgerTimeWait{
		c:     make(chan data.RippleTime),
		until: *until,
	}
	heap.Push(sub.timeListeners, listener)
	return listener.c
}

func (sub *Subscription) AfterTx(hash data.Hash256, min, max uint32) <-chan *websockets.TxResult {
	listener := txWait{
		hash: hash,
		c:    make(chan *websockets.TxResult),
		min:  min,
		max:  max,
	}

	sub.txListeners.PushBack(&listener)
	return listener.c
}

func (sub *Subscription) Ledgers() (uint32, uint32, error) {
	// block until we get a ledger event from the server.
	select {
	case <-sub.connected:
	}
	sub.mutex.RLock()
	defer sub.mutex.RUnlock()

	if sub.min == 0 || sub.max == 0 {
		return sub.min, sub.max, errors.New("Ledger history unknown")
	} else {
		return sub.min, sub.max, nil
	}
}

func (sub *Subscription) Loop() {
	for {
		// Message loop should continue indefinitely.
		sub.messageLoop()
		// However if it gets disconnected, try to reconnect.
		log.Println("Attempting reconnect...") // debug
		err := sub.connect()
		if err != nil {
			log.Println(err)
			// TODO wait with exponential backoff
			time.Sleep(10 * time.Second)
		} else {
			log.Println("...reconnected.") // debug
		}
	}
}

func (sub *Subscription) messageLoop() {
	// Consume messages as they arrive
	for {
		msg, ok := <-sub.Remote.Incoming
		if !ok {
			log.Printf("End subscription message loop to %s\n", sub.url) // verbose
			return
		} else {
		}

		switch msg := msg.(type) {
		case *websockets.LedgerStreamMsg:
			// msg.ValidatedLedgers is like "complete_ledgers" from server_info command.
			min, max, err := ParseCompleteLedgers(msg.ValidatedLedgers)
			if err == nil {
				sub.mutex.Lock()
				if sub.min == 0 && sub.max == 0 {
					// A closed channel never blocks.
					close(sub.connected)
				}
				sub.min = min
				sub.max = max

				// Inform anyone waiting on ledgers.
				for len(*sub.sequenceListeners) > 0 && (*sub.sequenceListeners)[0].until <= max {
					listener := heap.Pop(sub.sequenceListeners)
					listener.(ledgerSequenceWait).c <- max
					close(listener.(ledgerSequenceWait).c)
				}

				// inform anyone waiting on timestamp.
				for len(*sub.timeListeners) > 0 && (*sub.timeListeners)[0].until.Uint32() <= msg.LedgerTime.Uint32() {
					listener := heap.Pop(sub.timeListeners)
					listener.(ledgerTimeWait).c <- msg.LedgerTime
					close(listener.(ledgerTimeWait).c)
				}

				// Here we are notifying listeners about transactions.  We could instead subscribe to the tx feed.  TODO: would doing that be more efficient?
				//log.Printf("Looking for transaction in ledger %d\n", msg.LedgerSequence)
				//q.Q(msg)
				for l := sub.txListeners.Front(); l != nil; l = l.Next() {
					listener := l.Value.(*txWait)
					if max >= listener.min {
						select {
						case result := <-listener.r:

							// earlier request to remote.tx() has returned.
							q.Q(result) // debug
							listener.c <- result
							sub.txListeners.Remove(l)
							close(listener.c)

						default:
							if listener.r == nil {
								// initiate a request to remote.tx()
								go func(listener *txWait) {

									isLastTry := (min <= listener.min && max >= listener.max)

									result, err := sub.Remote.Tx(listener.hash)
									if err != nil {
										log.Println(err)
										listener.r = nil // We will try again, next ledger event.
									} else {
										// Let the listener know only about validated transactions.
										if result.Validated || isLastTry {
											// Note listener will receive either a validate transaction or not.
											listener.r = make(chan *websockets.TxResult)
											defer close(listener.r)
											listener.r <- result
										}
									}
								}(listener)
							}
						}
					}
				}

				sub.mutex.Unlock()

			}

			// Verbose
			//log.Printf("Ledger %d at %s with %d transactions.  Available ledger history: %d-%d\n", msg.LedgerSequence, msg.LedgerTime.String(), msg.TxnCount, min, max)

			// NOTE: the following message types not yet used/supported by this package!!!
		case *websockets.TransactionStreamMsg:
			terminal.Println(&msg.Transaction, terminal.Indent)
			for _, path := range msg.Transaction.PathSet() {
				terminal.Println(path, terminal.DoubleIndent)
			}
			trades, err := data.NewTradeSlice(&msg.Transaction)
			if err != nil {
				log.Println(err)
			} else {
				for _, trade := range trades {
					terminal.Println(trade, terminal.DoubleIndent)
				}
			}

			balances, err := msg.Transaction.Balances()
			if err != nil {
				log.Println(err)
			} else {
				for _, balance := range balances {
					terminal.Println(balance, terminal.DoubleIndent)
				}
			}
		case *websockets.ServerStreamMsg:
			terminal.Println(msg, terminal.Default)
		}
	}

}

/*
func (sub *Subscription) SignSubmitWait(t data.Transaction, keypair Keypair) (rpc.TxResult, error) {
TODO
}
*/
type ledgerSequenceWait struct {
	until uint32
	c     chan uint32
}

// An ledgerSequenceWaitHeap is a collection of channels waiting on ledger sequence to pass.
type ledgerSequenceWaitHeap []ledgerSequenceWait

func (h ledgerSequenceWaitHeap) Len() int           { return len(h) }
func (h ledgerSequenceWaitHeap) Less(i, j int) bool { return h[i].until < h[j].until }
func (h ledgerSequenceWaitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *ledgerSequenceWaitHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(ledgerSequenceWait))
}

func (h *ledgerSequenceWaitHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type ledgerTimeWait struct {
	until data.RippleTime
	c     chan data.RippleTime
}

// An ledgerTimeWaitHeap is a collection of channels waiting on ledger time to pass.
type ledgerTimeWaitHeap []ledgerTimeWait

func (h ledgerTimeWaitHeap) Len() int           { return len(h) }
func (h ledgerTimeWaitHeap) Less(i, j int) bool { return h[i].until.Uint32() < h[j].until.Uint32() }
func (h ledgerTimeWaitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *ledgerTimeWaitHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(ledgerTimeWait))
}

func (h *ledgerTimeWaitHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// txWait specifies waiting on a transaction to be validated.
type txWait struct {
	hash     data.Hash256
	min, max uint32 // tx LastLedgerSequence
	c        chan *websockets.TxResult
	r        chan *websockets.TxResult
}
