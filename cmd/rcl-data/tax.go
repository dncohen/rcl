package main

import (
	"container/heap"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/rippledata"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/y0ssar1an/q"
)

// View activity by account, with emphasis on cost basis of trades.

var (
	taxBaseAsset  data.Asset
	taxDataClient rippledata.Client
	taxMyAccounts map[data.Account]string // When we need to check if an account is ours.

	// Per-asset, chronological queue of basis price of historical acquisitions
	taxBasisQueue map[data.Asset]*rippledata.TransactionQueue

	// format detailed output table
	taxOutputTable *tabwriter.Writer

	// periodic totals
	monthlyTotals totals
	yearlyTotals  totals
	totalTotals   totals

	longTerm = time.Hour * 24 * 365 // Approximate test long term vs short term gain
)

func init() {
	taxMyAccounts = make(map[data.Account]string)
	taxBasisQueue = make(map[data.Asset]*rippledata.TransactionQueue)

	taxOutputTable = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
	writeHeader()
	totalTotals = newTotals("TOTAL")
}

// Running totals, normalized
type totals struct {
	name          string
	credit        *data.Value
	debit         *data.Value
	longTermGain  *data.Value
	shortTermGain *data.Value
}

func newTotals(name string) totals {
	return totals{
		name:          name,
		credit:        zeroNonNative.Clone(),
		debit:         zeroNonNative.Clone(),
		longTermGain:  zeroNonNative.Clone(),
		shortTermGain: zeroNonNative.Clone(),
	}
}

// Output
func writeHeader() {
	// Two rows for each event, for readability on terminal.
	//fmt.Fprintln(taxOutputTable, "Date\t Transaction\t Account\t Credit\t Debit\t Basis\t Notes ")
	fmt.Fprintln(taxOutputTable, "Date / \t Credit /\t Normalized /\t Basis\t Transaction")
	fmt.Fprintln(taxOutputTable, "Account\t Debit   \t             \t      \t Notes")
	fmt.Fprintln(taxOutputTable, "=======\t ========\t ============\t =====\t ===========")
}

func writeTotals(t totals) {
	if t.name == "" {
		// Not initialized totals
		return
	}
	fmt.Printf("\n\n%s credits %s %s, debits (%s %s)\n", t.name, t.credit, taxBaseAsset.Currency, t.debit, taxBaseAsset.Currency)
	fmt.Printf("%s long term gains: %s %s\n", t.name, t.longTermGain, taxBaseAsset.Currency)
	fmt.Printf("%s short term gains: %s %s\n\n", t.name, t.shortTermGain, taxBaseAsset.Currency)
}

func tallyDebit(amount data.Amount, normalized rippledata.NormalizeResponse, event accountTxRecord, basis []accountBasisRecord, notes string) {

	month := event.GetExecutedTime().Format("2006-01")
	if monthlyTotals.name != month {
		writeTotals(monthlyTotals)
		monthlyTotals = newTotals(month)
	}
	year := event.GetExecutedTime().Format("2006")
	if yearlyTotals.name != year {
		writeTotals(yearlyTotals)
		yearlyTotals = newTotals(year)
	}

	totalBasis := zeroNonNative.Clone()
	var err error
	for _, b := range basis {
		totalBasis, err = totalBasis.Add(b.Converted.Value)
		if err != nil {
			q.Q(err, totalBasis, b)
			log.Panic(err)
		}
	}
	basisWhen := "unknown"
	basisStr := ""
	if zeroNonNative.Less(*totalBasis) {
		if len(basis) > 1 {
			basisWhen = fmt.Sprintf("%s - %s", basis[0].GetExecutedTime().Format("2006-01-02"), basis[len(basis)-1].GetExecutedTime().Format("2006-01-02"))
		} else {
			basisWhen = basis[0].GetExecutedTime().Format("2006-01-02")
		}
		basisStr = fmt.Sprintf("%s %s", totalBasis, taxBaseAsset.Currency)

		// totals, we treat every debit like a sale.  It might actually be
		// a move say to another wallet or exchange.

		monthlyTotals.debit, _ = monthlyTotals.debit.Add(*totalBasis)
		yearlyTotals.debit, _ = yearlyTotals.debit.Add(*totalBasis)
		totalTotals.debit, _ = totalTotals.debit.Add(*totalBasis)
		gain, err := normalized.Converted.Subtract(*totalBasis)
		if err != nil {
			q.Q(err, normalized.Converted, totalBasis)
			log.Panic(err)
		}

		term := event.GetExecutedTime().Sub(basis[len(basis)-1].GetExecutedTime())
		if term < longTerm {
			monthlyTotals.shortTermGain, _ = monthlyTotals.shortTermGain.Add(*gain)
			yearlyTotals.shortTermGain, _ = yearlyTotals.shortTermGain.Add(*gain)
			totalTotals.shortTermGain, _ = totalTotals.shortTermGain.Add(*gain)
		} else {
			monthlyTotals.longTermGain, _ = monthlyTotals.longTermGain.Add(*gain)
			yearlyTotals.longTermGain, _ = yearlyTotals.longTermGain.Add(*gain)
			totalTotals.longTermGain, _ = totalTotals.longTermGain.Add(*gain)
		}
	}

	// Output to table
	fmt.Fprintf(taxOutputTable, "%s\t %s  \t %s  \t %s\t %s\n", event.GetExecutedTime().Format("2006-01-02"), "", "", basisWhen, event.GetHash())
	fmt.Fprintf(taxOutputTable, "%s\t (%s)\t (%s)\t %s\t %s\n", event.nick, config.FormatAmount(amount), normalized.Converted, basisStr, notes)
	fmt.Fprintln(taxOutputTable, "---\t---\t---\t---\t---") // blank
}

func tallyCredit(amount data.Amount, normalized rippledata.NormalizeResponse, event accountTxRecord, notes string) {

	month := event.GetExecutedTime().Format("2006-01")
	if monthlyTotals.name != month {
		writeTotals(monthlyTotals)
		monthlyTotals = newTotals(month)
	}
	year := event.GetExecutedTime().Format("2006")
	if yearlyTotals.name != year {
		writeTotals(yearlyTotals)
		yearlyTotals = newTotals(year)
	}

	monthlyTotals.credit, _ = monthlyTotals.credit.Add(normalized.Converted.Value)
	yearlyTotals.credit, _ = yearlyTotals.credit.Add(normalized.Converted.Value)
	totalTotals.credit, _ = totalTotals.credit.Add(normalized.Converted.Value)

	//fmt.Fprintf(taxOutputTable, "%d\t %s %s\n", rowNum, event.GetHash(), event.GetExecutedTime()) // no trailing tab
	//fmt.Fprintf(taxOutputTable, "%s\t %s\t %s\t %s\t \t %s\t %s\n", event.GetExecutedTime().Format("2006-01-02"), event.GetHash(), event.nick, config.FormatAmount(amount), "TODO", normalized.Converted.String()+" USD "+notes)
	fmt.Fprintf(taxOutputTable, "%s\t %s\t %s\t %s\t %s\n", event.GetExecutedTime().Format("2006-01-02"), config.FormatAmount(amount), normalized.Converted, "n/a credit", event.GetHash())
	fmt.Fprintf(taxOutputTable, "%s\t %s\t %s\t %s\t %s\n", event.nick, "", "", "", notes)
	fmt.Fprintln(taxOutputTable, "---\t---\t---\t---\t---") // blank
}

// Helper.  Any account listed in the command line args is considered "mine".
func accountIsMine(account data.Account) bool {
	_, ok := taxMyAccounts[account]
	//log.Printf("accountIsMine(%s) returning %t", account, ok) // debug
	return ok
}

// We'll be making a chronologically ordered queue of transaction records.
type accountTxRecord struct {
	rippledata.Transaction
	nick    string
	account *data.Account
}

// We'll be making chronologically ordered queues of cost basis information.
// There will be a queue per currency.
type accountBasisRecord struct {
	rippledata.NormalizeResponse
	accountTxRecord
}

func (s *State) tax(args ...string) {
	const help = `

The tax subcommand estimates gains based on historical ledger activity.

Usage:

  rcl-data tax <account> [<account>...]

When a transaction credits or debits an account, this command queries
data api for cost basis information.  Debits are considered sales, and
gains and losses are calculated against earlier credits, using
"first-in, first-out" order.

Same-currency payments between accounts listed on the command line are
ignored.  (i.e. moving funds, not a buy or sell)

Debits that are transaction fees are ignored.
  
`

	// subcommand-specific flags
	fs := flag.NewFlagSet("tax", flag.ExitOnError)

	// TODO accept "since" a point in calendar time or ledger sequence
	//fs.Int("since", 0, "Ignore ledger history before this ledger. Defaults to oldest available.")
	// TODO accept "until" range

	s.ParseFlags(fs, args, help, "show")

	s.taxCommand(fs)
}

func (s *State) taxCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " tax: ")

	// TODO make base asset configurable
	tmp, err := data.NewAmount("0/USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B") // rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B is bitstamp
	if err != nil {
		s.Exit(err)
	}
	taxBaseAsset = *tmp.Asset()

	accounts, err := accountsFromArgs(fs.Args())
	if err != nil {
		s.Exit(err)
	}

	for nick, account := range accounts {
		taxMyAccounts[*account] = nick // nick is whatever was typed on command line, a nickname or an address.
	}

	dataAPI := "https://data.ripple.com/v2/"           // trailing slash needed.
	taxDataClient, err = rippledata.NewClient(dataAPI) // trailing slash matters!
	if err != nil {
		s.Exit(err)
	}
	//log.Printf("Calculating %d account(s) %s via %s \n", len(accounts), accounts, dataAPI)

	var g errgroup.Group

	// Who created each account?
	accountData := make(map[string]rippledata.AccountData)
	for nick, account := range accounts {
		account := account // scope!
		nick := nick
		g.Go(func() error {

			response, err := taxDataClient.AccountData(*account)
			if err != nil {
				return err
			}
			//log.Printf("Got account data for %s = %s", nick, account)
			accountData[nick] = response.AccountData
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}
	for nick, data := range accountData {
		log.Printf("%s created by %s at %s", nick, config.FormatAccountName(data.Parent), data.Inception)
	}

	// While developing, limit number of tx processed.
	develLimit := 0 // 0 for no limit
	develCount := 0

	// loop over all history.

	// Originally, We did not use balance_changes endpoint
	// because it does not return enough information about each event.
	// Instead we get payments and exchanges both.

	// We put the oldest payment and oldest exchange for each account
	// into a heap (orders them chronologically).  We pull the oldest
	// event from the heap, then add next event from that same account
	// to the heap, repeat until all events processed.

	// However, some payments get returned not only as a payment but
	// also as many exchanges!  As if our account held CNY.ripplefox and
	// other issuances that we don't even have a trust line for!

	// So long story, now back to using balance_changes and hoping we can make it work.

	//paymentChannels := make(map[string]chan rippledata.Payment)
	//exchangeChannels := make(map[string]chan rippledata.Exchange)

	balanceChangeChannels := make(map[string]chan rippledata.BalanceChangeDescriptor)

	pendingQueue := make(rippledata.TransactionQueue, 0, len(accounts)*2)
	heap.Init(&pendingQueue) // ordered transactions to be processed.

	// Initialize channels for each account.
	for nick, account := range accounts {
		//paymentChannels[nick] = taxDataClient.GetAccountPaymentsAsync(*account)
		//exchangeChannels[nick] = taxDataClient.GetAccountExchangesAsync(*account)
		balanceChangeChannels[nick] = taxDataClient.GetBalanceChangesAsync(*account)
	}

	/*
		// Helpers to pull from channels onto queue
		queueNextPayment := func(nick string) {
			//log.Printf("Queueing next payment affecting %s...", nick) // debug
			payment, ok := <-paymentChannels[nick]
			if ok {
				heap.Push(&pendingQueue, &accountTxRecord{
					Transaction: &payment,
					nick:        nick,
					account:     accounts[nick],
				})
				//log.Printf("... got next payment affecting %s. Pending queue has %d", nick, pendingQueue.Len())
			} else {
				log.Printf("no more payments affecting %s", nick)
			}
		}
		queueNextExchange := func(nick string) {
			exchange, ok := <-exchangeChannels[nick]
			if ok {
				heap.Push(&pendingQueue, &accountTxRecord{
					Transaction: &exchange,
					nick:        nick,
					account:     accounts[nick],
				})
			}
		}
	*/
	queueNextBalanceChange := func(nick string) {
		event, ok := <-balanceChangeChannels[nick]
		if ok {
			heap.Push(&pendingQueue, &accountTxRecord{
				Transaction: event,
				nick:        nick,
				account:     accounts[nick],
			})
		}
	}

	// Thought this would simplify... maybe not.  We might be better off with above.
	queueNext := func(record accountTxRecord) {
		var ok bool
		var event rippledata.Transaction
		switch record.Transaction.(type) {
		case rippledata.BalanceChangeDescriptor:
			event, ok = <-balanceChangeChannels[record.nick]
		default:
			log.Panicf("Unexpected record type %T", record.Transaction)
		}
		if ok {
			heap.Push(&pendingQueue, &accountTxRecord{
				Transaction: event,
				nick:        record.nick,
				account:     record.account,
			})
		}
	}

	// Initialize the heap with the first record from each account.
	for nick, _ := range accounts {
		//queueNextPayment(nick)
		//queueNextExchange(nick)
		queueNextBalanceChange(nick)
	}

	// In order to ignore payments from our account to our account, we
	// need to collect all changes before processing any in transaction.
	eventsBySingleTx := make([]*accountTxRecord, 0, 2)

	// Until done, pull oldest tx from heap.
oldestTxLoop:
	for (develLimit == 0 || develCount < develLimit) && (pendingQueue.Len() > 0 || len(eventsBySingleTx) > 0) {
		record := heap.Pop(&pendingQueue).(*accountTxRecord)

		// Every time we pull an event off the pending queue, put the next event for the same account onto the queue.
		queueNext(*record) // We'll process the next one during later iteration of oldestTxLoop

		// configuration serves as a place to store notes about transactions.
		section, _ := config.GetSection(record.GetHash().String())
		if section != nil {
			// TODO put note into table
			log.Println(section.Name(), section.Key("note"))
		}

		// Before processing a balance change, we want to check is this
		// a payment from our account to another of our accounts?  We
		// take advantage of the fact that if the tx affects two
		// accounts, it will still be on the top of the queue.
		if len(eventsBySingleTx) == 0 {
			eventsBySingleTx = append(eventsBySingleTx, record)
		} else if record.GetHash() == eventsBySingleTx[0].GetHash() {
			// Another event from same tx
			eventsBySingleTx = append(eventsBySingleTx, record)
		}

		if pendingQueue.Len() > 0 && pendingQueue[0].GetHash() == eventsBySingleTx[0].GetHash() {
			continue oldestTxLoop
		}
		// Here, there are no further events from this same transaction.  We can process the events we have gathered.

		// TODO check are we the sender and receiver?
		var source, destination *accountTxRecord
		//source, destination = nil, nil
		for _, record := range eventsBySingleTx {
			switch event := record.Transaction.(type) {
			case rippledata.BalanceChangeDescriptor:
				if event.ChangeType == "payment_source" {
					source = record
				}
				if event.ChangeType == "payment_destination" {
					destination = record
				}
			}
		}

		if source != nil && destination != nil {
			sourceCurrency := source.Transaction.(rippledata.BalanceChangeDescriptor).Currency
			destCurrency := destination.Transaction.(rippledata.BalanceChangeDescriptor).Currency
			if sourceCurrency == destCurrency {
				log.Printf("Ignoring %d balance changes caused by payment from %s to %s for %s \t %s", len(eventsBySingleTx), source.nick, destination.nick, config.FormatAmount(*destination.Transaction.(rippledata.BalanceChangeDescriptor).GetChangeAmount()), destination.GetHash())

				// discard events we've collected
				eventsBySingleTx = make([]*accountTxRecord, 0, 2)
				continue oldestTxLoop
			} else {
				log.Printf("Processing payment which sent %s from %s, delivered %s to %s, %s", source.Transaction.(rippledata.BalanceChangeDescriptor).GetChangeAmount(), source.nick, destination.Transaction.(rippledata.BalanceChangeDescriptor).GetChangeAmount(), destination.nick, destination.GetHash())
			}
		}

		// log.Printf("processing %d events from %s", len(eventsBySingleTx), eventsBySingleTx[0].GetHash()) // verbose!

	currentTxLoop:
		for _, record := range eventsBySingleTx {
			switch event := record.Transaction.(type) {
			case rippledata.BalanceChangeDescriptor:

				if event.ChangeType == "transaction_cost" {
					// ignore
					//queueNext(*record) queueing is now in eventsBySingleTx loop.
					continue currentTxLoop
				}

				processBalanceChangeDescriptor(*record, event)
				//queueNextBalanceChange(record.nick)

			case *rippledata.Payment:

				if event.SourceCurrency == event.DestinationCurrency &&
					accountIsMine(event.Source) && accountIsMine(event.Destination) {
					log.Printf("Ignoring payment %s %s from %s to %s", event.DeliveredAmount, event.DestinationCurrency, taxMyAccounts[event.Source], taxMyAccounts[event.Destination])
					//queueNext(*record)
					continue currentTxLoop
				}

				//log.Printf("payment affects %s in ledger %d %s", record.nick, event.GetLedgerIndex(), event.GetHash())

				// data api doesn't actually return issuing address of source or destination amounts!  So better to iterate through balance changes.
				if event.Source == *record.account {
					// Our account was source of payment
					processBalanceChangeObjects(*record, event.SourceBalanceChanges)
				}
				if event.Destination == *record.account {
					// Our account was destination of payment
					processBalanceChangeObjects(*record, event.DestinationBalanceChanges)
				}

				// Put the next event for this account into the queue.
				//queueNext(*record)

			case *rippledata.Exchange:
				// log.Printf("Exchange affects %s in ledger %d %s", record.nick, event.GetLedgerIndex(), event.GetHash()) //verbose
				processExchange(*record, event)
				//queueNext(*record)

			default:
				log.Printf("unhandled event %T in ledger %d %s", event, event.GetLedgerIndex(), event.GetHash())
			}

			//
			develCount++
		}

		// move on to next tx....
		eventsBySingleTx = make([]*accountTxRecord, 0, 2)

	}

	writeTotals(monthlyTotals)
	writeTotals(yearlyTotals)
	writeTotals(totalTotals)
	log.Printf("Exiting after %d events with %d events in queue", develCount, pendingQueue.Len())
	taxOutputTable.Flush()

	writeTotals(totalTotals)

	log.Printf("Exiting after %d events with %d events in queue", develCount, pendingQueue.Len())
}

// Add purchase information to history, to be used as cost basis for later sale.
func pushBasis(amount data.Amount, normalized rippledata.NormalizeResponse, record accountTxRecord) {

	//log.Printf("%s acquired %s, basis %s %s", record.nick, amount, normalized.Converted, taxBaseAsset) // debug

	asset := amount.Asset()
	queue, ok := taxBasisQueue[*asset]
	if !ok {
		queue = rippledata.NewTransactionQueue()
		heap.Init(queue)
		taxBasisQueue[*asset] = queue
	}
	heap.Push(queue, accountBasisRecord{
		NormalizeResponse: normalized,
		accountTxRecord:   record,
	})
	return
}

// Retrieve cost basis for sale information.
func popBasisHistory(delta data.Amount) []accountBasisRecord {
	// log.Printf("popBasisHistory(%s)", delta) // debug
	response := make([]accountBasisRecord, 0, 1)

	// Sanity
	if delta.Less(*zeroNonNative) {
		log.Panicf("popBasisHistory expected positive amount, got %s", delta)
	}

	// pop as many times as needed from basis history to account for
	// full amount.  If history accounts for more than this amount, push
	// the remainder back onto history.
	asset := delta.Asset()
	remaining, err := delta.Value.Ratio(*oneNonNative) // For our math, use non-native event when XRP.
	if err != nil {
		log.Panic(err)
	}
	queue, ok := taxBasisQueue[*asset]
	if !ok || queue.Len() < 1 {
		//log.Printf("No basis history for %s", asset) // verbose
		return response
	}

	var history accountBasisRecord
	for queue.Len() > 0 && zeroNonNative.Less(*remaining) {
		//log.Printf("popBasisHistory remaining: %s", remaining)
		history = heap.Pop(queue).(accountBasisRecord)
		remaining, err = remaining.Subtract(history.Amount.Value)
		if err != nil {
			q.Q(delta)
			q.Q(remaining, history.NormalizeResponse)
			log.Panic(err)
		}
		//log.Printf("popped %s from %s basis history", history.Amount, asset)
		response = append(response, history)
	}

	if remaining.IsNegative() {
		//q.Q("history before deduction", history.NormalizeResponse) // debug
		remainder := remaining.Negate()
		history.Amount.Value = *remainder
		tmp, err := history.Amount.Multiply(history.Rate.Value) // multiply or divide?
		if err != nil {
			q.Q(err, history.NormalizeResponse)
			log.Panic(err)
		}
		history.Converted.Value = *tmp
		//q.Q("history after deduction", history.NormalizeResponse)

		// Correct the last entry in our response
		lastResponse := &response[len(response)-1]

		correctAmount, err := lastResponse.Amount.Subtract(history.Amount.Value)
		if err != nil {
			q.Q(err, lastResponse.NormalizeResponse, history.NormalizeResponse)
			log.Panic(err)
		}
		lastResponse.Amount.Value = *correctAmount

		correctConverted, err := lastResponse.Converted.Subtract(history.Converted.Value)
		if err != nil {
			q.Q(err, lastResponse.NormalizeResponse, history.NormalizeResponse)
			log.Panic(err)
		}
		lastResponse.Converted.Value = *correctConverted

		// Put remainder back on queue
		heap.Push(queue, history)

		//log.Printf("returned %s to %s basis history", history.Amount, asset)
	}
	return response
}

func processBalanceChangeDescriptor(record accountTxRecord, change rippledata.BalanceChangeDescriptor) error {
	delta := change.GetChangeAmount()
	if delta.Less(*zeroNonNative) {
		// Something we sent or sold.
		delta = delta.Abs() // Convert to positive (exchange records hold only positive amounts)
		normalized, err := taxDataClient.Normalize(*delta, taxBaseAsset, record.GetExecutedTime())
		if err != nil {
			return errors.Wrapf(err, "Failed to get cost basis for %s in transaction %s", delta, record.GetHash())
		}

		basis := popBasisHistory(*delta)

		tallyDebit(*delta, *normalized, record, basis, fmt.Sprintf("%s", change.ChangeType))
	} else {
		// Something we received or bought.
		normalized, err := taxDataClient.Normalize(*delta, taxBaseAsset, record.GetExecutedTime())
		if err != nil {
			return errors.Wrapf(err, "Failed to get cost basis for %s in transaction %s", delta, record.GetHash())
		}

		pushBasis(*delta, *normalized, record)
		tallyCredit(*delta, *normalized, record, fmt.Sprintf("%s", change.ChangeType))
	}
	return nil
}

// Note that "balance change objects" are not the same thing as "balance change descriptors" in data API.  Yikes!
// https://ripple.com/build/data-api-v2/#balance-objects-and-balance-change-objects
func processBalanceChangeObjects(record accountTxRecord, changes []rippledata.BalanceChangeObject) error {
	payment := record.Transaction.(*rippledata.Payment)
	from, _ := config.GetAccountNickname(payment.Source)

	to, _ := config.GetAccountNickname(payment.Destination)

	for _, change := range changes {
		delta := change.GetAmount()

		if delta.Less(*zeroNonNative) {
			// Something we sent or sold.
			delta = delta.Abs() // Convert to positive (exchange records hold only positive amounts)
			normalized, err := taxDataClient.Normalize(*delta, taxBaseAsset, record.GetExecutedTime())
			if err != nil {
				return errors.Wrapf(err, "Failed to get cost basis for %s in transaction %s", delta, record.GetHash())
			}

			basis := popBasisHistory(*delta) // TODO make this function return something useful

			tallyDebit(*delta, *normalized, record, basis, fmt.Sprintf("Sent %s %s to %s from %s", payment.DeliveredAmount, payment.DestinationCurrency, to, from))
		} else {
			// Something we received or bought.
			normalized, err := taxDataClient.Normalize(*delta, taxBaseAsset, record.GetExecutedTime())
			if err != nil {
				return errors.Wrapf(err, "Failed to get cost basis for %s in transaction %s", delta, record.GetHash())
			}

			pushBasis(*delta, *normalized, record)
			tallyCredit(*delta, *normalized, record, fmt.Sprintf("Received %s %s to %s from %s", payment.DeliveredAmount, payment.DestinationCurrency, to, from))
		}
	}
	return nil
}

func processExchange(record accountTxRecord, exchange *rippledata.Exchange) error {

	baseAmount := exchange.GetBaseAmount()
	normalizedBase, err := taxDataClient.Normalize(*baseAmount, taxBaseAsset, record.GetExecutedTime())
	if err != nil {
		return errors.Wrapf(err, "Failed to normalize %s in transaction %s", baseAmount, record.GetHash())
	}
	counterAmount := exchange.GetCounterAmount()
	normalizedCounter, err := taxDataClient.Normalize(*counterAmount, taxBaseAsset, record.GetExecutedTime())
	if err != nil {
		return errors.Wrapf(err, "Failed to normalize %s in transaction %s", counterAmount, record.GetHash())
	}

	if exchange.Buyer == *record.account {
		// acquired the base currency
		// log.Printf("%s sold %s, acquired %s", record.nick, counterAmount, baseAmount) // verbose
		pushBasis(*baseAmount, *normalizedBase, record)
		tallyCredit(*baseAmount, *normalizedBase, record, "trade acquired")
		basis := popBasisHistory(*counterAmount)
		tallyDebit(*counterAmount, *normalizedCounter, record, basis, "trade sold")
	}

	if exchange.Seller == *record.account {
		//log.Printf("%s sold %s, acquired %s", record.nick, baseAmount, counterAmount)
		pushBasis(*counterAmount, *normalizedCounter, record)
		tallyCredit(*counterAmount, *normalizedCounter, record, "trade acquired")
		basis := popBasisHistory(*baseAmount)
		tallyDebit(*baseAmount, *normalizedBase, record, basis, "trade sold")
	}
	return nil
}
