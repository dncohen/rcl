package main

import (
	"container/heap"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/dncohen/rcl/rippledata"
	"github.com/dncohen/rcl/rippledata/history"
	"github.com/rubblelabs/ripple/data"
)

// View activity by account, with emphasis on cost basis of trades.

var (
	taxBaseAsset data.Asset
	taxBaseOne   data.Amount // 1.0 of the base currency

	taxDataClient rippledata.Client
	taxMyAccounts map[data.Account]string // When we need to check if an account is ours.

	useFIFO bool // FIFO, otherwise LIFO

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
	//taxBasisQueue = make(map[data.Asset]heap.Interface)

	taxOutputTable = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
	writeHeader()
}

// Normalized (converted to another currency) cost basis of a transaction
type TaxInventory struct {
	history.AccountTx
	rate      data.NonNativeValue // normalized rate vs basis asset
	inventory *data.Amount        // Amount acquired and not yet traded (unpsent)

}

func (this TaxInventory) Clone() *TaxInventory {
	copy := TaxInventory{
		AccountTx: this.AccountTx,
		rate:      this.rate,
		inventory: this.inventory.Clone(),
	}
	return &copy
}

func (this TaxInventory) CostBasis() *data.Amount {
	this.inventory.Value = mustValue(this.inventory.Value.Ratio(*taxBaseOne.Value)) // Convert native to non-native!

	amount, err := taxBaseOne.Clone().Multiply(this.inventory)
	if err != nil {
		log.Panic(err)
	}
	amount.Value, err = amount.Value.Multiply(this.rate.Value)
	if err != nil {
		log.Panic(err)
	}

	// debug, ensure native vs non-native math is correct.
	//log.Printf("cost basis: %s * %s * %s = %s", taxBaseOne, this.inventory, this.rate, amount)

	return amount

}

// Running totals normalized to base currency
type totals struct {
	name          string // for rendering table
	credit        *data.Amount
	debit         *data.Amount
	longTermGain  *data.Amount
	shortTermGain *data.Amount
}

func newTotals(name string) totals {
	return totals{
		name:          name,
		credit:        taxBaseOne.ZeroClone(),
		debit:         taxBaseOne.ZeroClone(),
		longTermGain:  taxBaseOne.ZeroClone(),
		shortTermGain: taxBaseOne.ZeroClone(),
	}
}

// Output
func writeHeader() {
	// Two rows for each event, for readability on terminal.
	fmt.Fprintln(taxOutputTable, "Date / \t Credit /\t Normalized /\t Basis\t Transaction")
	fmt.Fprintln(taxOutputTable, "Account\t Debit   \t              \t      \t Notes")
	fmt.Fprintln(taxOutputTable, "=======\t ========\t ============\t =====\t ===========")
}

func writeTotals(t totals) {
	if t.name == "" {
		// Not initialized totals
		// this is reached when we first start to tally
		return
	}
	fmt.Printf("\n\n%s credits %s %s, debits (%s %s)\n", t.name, t.credit, taxBaseAsset.Currency, t.debit, taxBaseAsset.Currency)
	fmt.Printf("%s long term gains: %s %s\n", t.name, t.longTermGain, taxBaseAsset.Currency)
	fmt.Printf("%s short term gains: %s %s\n\n", t.name, t.shortTermGain, taxBaseAsset.Currency)
}

func tallyDebit(amount data.Amount, rate data.NonNativeValue, event history.AccountTx, inventory []TaxInventory, notes string) {
	// amount is negative
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

	totalBasis := taxBaseOne.ZeroClone()
	var err error

	for i, basis := range inventory {
		log.Printf("Consuming cost basis %d %s", i, basis.CostBasis())
		totalBasis, err = totalBasis.Add(basis.CostBasis())
		if err != nil {
			log.Panic(err)
		}
	}

	basisWhen := fmt.Sprintf("%s - %s", inventory[0].GetExecutedTime().Format("2006-01-02"), inventory[len(inventory)-1].GetExecutedTime().Format("2006-01-02"))

	monthlyTotals.debit = mustAmount(monthlyTotals.debit.Add(totalBasis))
	yearlyTotals.debit = mustAmount(yearlyTotals.debit.Add(totalBasis))
	totalTotals.debit = mustAmount(totalTotals.debit.Add(totalBasis))

	normalizedAmount := normalizeAmount(amount, rate) // a negative number

	// Gain is the difference between basis vs todays price
	gain, err := totalBasis.Add(normalizedAmount)
	if err != nil {
		log.Panic(err)
	}
	gain = gain.Negate() // result is negative when we have a gain, hence the Negate()

	// verbose debug
	log.Println("amount:", amount)
	log.Println("normalizedAmount:", normalizedAmount)
	log.Println("totalBasis:", totalBasis)
	log.Println("Gain:", gain)

	// Sanity
	// Note that a loss (negative gain) can be greater than the normalized amount.  But a gain should never be.
	if normalizedAmount.Abs().Less(*gain.Value) {
		log.Panicf("Calculated gain %s greater than transaction amount %s normalized to %s", gain, amount, normalizedAmount)
	}

	term1 := event.GetExecutedTime().Sub(inventory[0].GetExecutedTime())
	term2 := event.GetExecutedTime().Sub(inventory[len(inventory)-1].GetExecutedTime())
	if term1 < longTerm || term2 < longTerm {
		notes = fmt.Sprintf("%s (short term gain %s)", notes, gain.Value)
		monthlyTotals.shortTermGain = mustAmount(monthlyTotals.shortTermGain.Add(gain))
		yearlyTotals.shortTermGain = mustAmount(yearlyTotals.shortTermGain.Add(gain))
		totalTotals.shortTermGain = mustAmount(totalTotals.shortTermGain.Add(gain))
	} else {
		notes = fmt.Sprintf("%s (long term gain %s)", notes, gain.Value)
		monthlyTotals.longTermGain = mustAmount(monthlyTotals.longTermGain.Add(gain))
		yearlyTotals.longTermGain = mustAmount(yearlyTotals.longTermGain.Add(gain))
		totalTotals.longTermGain = mustAmount(totalTotals.longTermGain.Add(gain))
	}
	// Output to table
	fmt.Fprintf(taxOutputTable, "%s\t %s  \t %s  \t %s\t %s\n", event.GetExecutedTime().Format("2006-01-02"), "", "", basisWhen, event.GetHash())
	fmt.Fprintf(taxOutputTable, "%s\t (%s)\t (%s)\t %s\t %s\n", event.Nick, config.FormatAmount(amount), normalizedAmount.Value, totalBasis.Value, notes)
	fmt.Fprintln(taxOutputTable, "---\t---\t---\t---\t---") // blank

}

func mustValue(v *data.Value, err error) *data.Value {
	if err != nil {
		log.Panic(err)
	}
	return v
}
func mustAmount(v *data.Amount, err error) *data.Amount {
	if err != nil {
		log.Panic(err)
	}
	return v
}

func normalizeAmount(amount data.Amount, rate data.NonNativeValue) *data.Amount {
	// sanity
	if amount.Asset().String() == taxBaseOne.Asset().String() {
		log.Printf("Normalizing base asset amount %s at rate %s", amount, rate)
		if !rate.Equals(*taxBaseOne.Value) {
			log.Panicf("rate %s is not one.", rate)
		}
	}

	amount.Value = mustValue(amount.Value.Ratio(*taxBaseOne.Value)) // converts a native value to non-native.

	// Calculate amount normalized to today's price
	normalizedAmount := taxBaseOne.Clone()
	normalizedAmount = mustAmount(normalizedAmount.Multiply(&amount))
	normalizedAmount.Value = mustValue(normalizedAmount.Value.Multiply(rate.Value))

	// debug
	//log.Printf("normalizeAmount(%s): %s * %s * %s = %s", amount, taxBaseOne, amount, rate, normalizedAmount)
	//log.Printf("normalizedAmount %s IsNative: %t", normalizedAmount, normalizedAmount.IsNative())

	return normalizedAmount
}

func tallyCredit(amount data.Amount, rate data.NonNativeValue, event history.AccountTx, notes string) {

	month := event.GetExecutedTime().Format("2006-01")
	if monthlyTotals.name != month {
		writeTotals(monthlyTotals)
		monthlyTotals = newTotals(month)
	}
	year := event.GetExecutedTime().Format("2006")
	if yearlyTotals.name != year {
		writeTotals(yearlyTotals)
		log.Println("tallying year:", year)
		yearlyTotals = newTotals(year)
	}

	normalizedAmount := normalizeAmount(amount, rate)
	monthlyTotals.credit = mustAmount(monthlyTotals.credit.Add(normalizedAmount))
	yearlyTotals.credit = mustAmount(yearlyTotals.credit.Add(normalizedAmount))
	totalTotals.credit = mustAmount(totalTotals.credit.Add(normalizedAmount))

	log.Println("Monthly total credit now", monthlyTotals.credit) // devel

	//fmt.Fprintf(taxOutputTable, "%d\t %s %s\n", rowNum, event.GetHash(), event.GetExecutedTime()) // no trailing tab
	//fmt.Fprintf(taxOutputTable, "%s\t %s\t %s\t %s\t \t %s\t %s\n", event.GetExecutedTime().Format("2006-01-02"), event.GetHash(), event.nick, config.FormatAmount(amount), "TODO", normalized.Converted.String()+" USD "+notes)
	fmt.Fprintf(taxOutputTable, "%s\t %s\t %s\t %s\t %s\n", event.GetExecutedTime().Format("2006-01-02"), config.FormatAmount(amount), normalizedAmount.Value, "n/a credit", event.GetHash())
	fmt.Fprintf(taxOutputTable, "%s\t %s\t %s\t %s\t %s\n", event.Nick, "", "", fmt.Sprintf("@ %s", rate), notes)
	fmt.Fprintln(taxOutputTable, "---\t---\t---\t---\t---") // blank
}

// Helper.  Any account listed in the command line args is considered "mine".
func accountIsMine(account data.Account) bool {
	_, ok := taxMyAccounts[account]
	return ok
}

// We'll be making a chronologically ordered queue of transaction records.

func (s *State) tax(args ...string) {
	const help = `

The tax subcommand estimates gains based on historical ledger activity.

Usage:

  rcl-data tax [-lifo] [-fifo] <account> [<account>...]

Specify one of -fifo or -lifo.  Determines whether cost basis is
calculated "first in, first out" or "last in, first out".

When a transaction credits or debits an account, this command queries
data api for cost basis information.  Debits are considered sales, and
gains and losses are calculated against earlier credits.

Same-currency payments between accounts listed on the command line are
ignored.  (i.e. moving funds, not a buy or sell)

Debits that are transaction fees are ignored.
  
`

	// subcommand-specific flags
	fs := flag.NewFlagSet("tax", flag.ExitOnError)

	// TODO accept "since" a point in calendar time or ledger sequence
	//fs.Int("since", 0, "Ignore ledger history before this ledger. Defaults to oldest available.")
	// TODO accept "until" range

	fs.Int("n", 0, "How many transactions to process (default: 0, meaning all)")

	fs.Bool("fifo", false, "Use first in, first out basis calculation")
	fs.Bool("lifo", false, "Use last in, first out basis calculation")

	s.ParseFlags(fs, args, help, "show")

	// Validate
	fifo := boolFlag(fs, "fifo")
	lifo := boolFlag(fs, "lifo")
	if fifo == lifo {
		s.Exitf("Specify either -lifo or -fifo")
	}

	s.taxCommand(fs)
}

func (s *State) taxCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " tax: ")

	useFIFO = boolFlag(fs, "fifo")
	limit := intFlag(fs, "n")

	// TODO make base asset configurable
	tmp, err := data.NewAmount("1.0/USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B") // rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B is bitstamp
	if err != nil {
		s.Exit(err)
	}
	taxBaseAsset = *tmp.Asset()
	taxBaseOne = *tmp // One unit of the base, i.e. 1 USD

	// Initialize totals after taxBaseOne.
	totalTotals = newTotals("TOTAL")

	// Per-asset, chronological queue of basis price of historical acquisitions
	pastInventory := make(map[data.Asset]heap.Interface)

	// RCL accounts to observe, from command line.
	accounts, err := accountsFromArgs(fs.Args())
	if err != nil {
		s.Exit(err)
	}

	// Whatever was typed on command line (full account or nickname from
	// config) is how we'll refer to the account in output.
	for nick, account := range accounts {
		taxMyAccounts[*account] = nick // nick is whatever was typed on command line, a nickname or an address.
	}

	dataAPI := "https://data.ripple.com/v2/"           // trailing slash needed.
	taxDataClient, err = rippledata.NewClient(dataAPI) // trailing slash matters!
	if err != nil {
		s.Exit(err)
	}
	//log.Printf("Calculating %d account(s) %s via %s \n", len(accounts), accounts, dataAPI)

	// Iterate over balance changes for each account
	var events []*history.AccountTx
	iterator := history.NewBalanceChangeIterator(taxDataClient, accounts)
	err = iterator.Init()
	if err != nil {
		s.Exit(err)
	}

	for nick, data := range iterator.AccountData {
		log.Printf("%s created by %s at %s", nick, config.FormatAccountName(data.Parent), data.Inception)
	}

	count := 0

	// loop over all history.

	// Originally, We did not use balance_changes endpoint
	// because it does not return enough information about each event.
	// Instead we get payments and exchanges both.

	// However, some payments get returned not only as a payment but
	// also as many exchanges!  As if our account held CNY.ripplefox and
	// other issuances that we don't even have a trust line for!

	// So long story, now back to using balance_changes and hoping we can make it work.

	// Iterator returns multiple "events" for each transaction.
	for events = iterator.Next(); events != nil; events = iterator.Next() {
		count++
		log.Println(events[0].GetExecutedTime().Format("2006-01-02"))
		log.Println(events[0].GetHash())
		log.Printf("Tx %d (%s) has %d events", count, events[0].GetHash(), len(events))

		// configuration serves as a place to store notes about transactions.
		notes := ""
		strict := false
		ignore := false
		section, _ := config.GetSection(events[0].GetHash().String())
		if section != nil {
			log.Println(section.Name(), section.Key("note"))
			notes = section.Key("note").String()
			ignore = section.Key("ignore").MustBool(false)
			strict = section.Key("strict").MustBool(false)
		}

		// Inspect the transaction...

		var source, destination *rippledata.BalanceChangeDescriptor
		var sourceEvent, destinationEvent *history.AccountTx
		for i, event := range events {
			log.Printf("event %d of %d: %T", i+1, len(events), event.Transaction)
			switch t := event.Transaction.(type) {
			case rippledata.BalanceChangeDescriptor:
				amount := t.GetChangeAmount()
				log.Printf("event %d of %d: %s %s", i+1, len(events), t.ChangeType, amount) // verbose

				switch typ := t.ChangeType; typ {
				case "payment_destination":
					destination = &t
					destinationEvent = event
				case "payment_source":
					source = &t
					sourceEvent = event
				}
			}
		}

		if ignore {
			log.Println("Ignoring transaction (due to configuration setting)", events[0].GetHash())
			continue
		}

		// Are we the sender and receiver?

		if source != nil && destination != nil {
			if source.Currency == destination.Currency && source.Counterparty == destination.Counterparty {
				log.Printf("Ignoring %d balance changes caused by payment from %s to %s for %s \t %s", len(events), sourceEvent.Nick, destinationEvent.Nick, config.FormatAmount(*destination.GetChangeAmount()), destinationEvent.GetHash())

				continue
			} else {
				log.Printf("Processing payment which sent %s from %s, delivered %s to %s, %s", source.GetChangeAmount(), sourceEvent.Nick, destination.GetChangeAmount(), destinationEvent.Nick, destination.GetHash())
			}
		}

		// process balance changes
		for _, event := range events {
			switch t := event.Transaction.(type) {
			case rippledata.BalanceChangeDescriptor:

				amount := t.GetChangeAmount()
				asset := amount.Asset()

				// Honor currency-specific configuration.  Used to ignore
				// experimental issuances or issuances that have to conversion
				// to base asset.
				section, _ := config.GetSection(asset.String())
				if section != nil {
					log.Println(section.Name(), section.Key("note"))
					if section.Key("ignore").MustBool(false) {
						log.Printf("Ignoring %s %s (configured to ignore %s)", t.ChangeType, amount, asset)
						continue
					}
				}

				if t.ChangeType == "transaction_cost" {
					// TODO: handle fees as expenses
				} else {
					if amount.IsZero() {
						// This can happen when an offercreate does not result in a trade.
						//log.Panicf("Unexpected zero amount, %s %s", t.ChangeType, event.GetHash())
						log.Printf("Ignoring zero amount change, %s %s %s", amount, t.ChangeType, event.GetHash())
						continue
					}

					if amount.IsNegative() {
						// Asset we spend or trade
						// Current price vs historic basis is our gain
						queue := pastInventory[*asset]
						inventory, remainder := popBasisHistory(*amount, queue)
						if !remainder.IsZero() {
							msg := fmt.Sprintf("popBasisHistory returned non-zero remainder (%s)", remainder)
							// Use configuration strict=false to allow specific
							// transactions to pass here.  This is useful because
							// some math can be off by small amounts, i.e.
							// -13e-17/BTC/rMwjYedjc7qqtKYVLiAccJSmCwih4LnE2q, and I
							// haven't actually figured out why!
							if strict {
								log.Panic(msg)
							} else {
								log.Println(msg)
							}
						}
						if len(inventory) == 0 {
							log.Panic("popBasisHistory return no history")
						} else {
							log.Printf("popBasisHistory accounted for %s in %d prior transactions, starting %s at %s", amount, len(inventory), inventory[0].inventory, inventory[0].rate)
						}

						normalized, err := normalize(*amount, taxBaseAsset, t.GetExecutedTime())
						if err != nil {
							log.Panic(err)
						}
						tallyDebit(*amount, normalized.Rate, *event, inventory, notes)

					} else {
						// Asset we purchased (or received)
						// Store current price as cost basis for later sale.
						normalized, err := normalize(*amount, taxBaseAsset, t.GetExecutedTime())
						if err != nil {
							log.Panic(err)
						}

						pastInventory[*asset] = pushBasis(pastInventory[*asset], *amount, *normalized, *event)
						tallyCredit(*amount, normalized.Rate, *event, notes)
					}

				}
			default:
				log.Panicf("Unexpected event type: %T", t)
			}
		}

		//
		if limit > 0 && count >= limit {
			log.Printf("Reached develLimit %d transactions", limit)
			break
		}
	}

	writeTotals(monthlyTotals)
	writeTotals(yearlyTotals)
	writeTotals(totalTotals)

	taxOutputTable.Flush()

	writeTotals(totalTotals)

}

// Add purchase information to history, to be used as cost basis for later sale.
func pushBasis(queue heap.Interface, amount data.Amount, normalized rippledata.NormalizeResponse, event history.AccountTx) heap.Interface {
	// sanity
	if amount.IsNegative() || amount.IsZero() {
		log.Panicf("pushBasis expected positive amount, got %s", amount)
	}

	if queue == nil {
		if useFIFO {
			queue = rippledata.NewTransactionFIFO()
		} else {
			queue = rippledata.NewTransactionLIFO()
		}
		heap.Init(queue)
	}
	heap.Push(queue, TaxInventory{
		AccountTx: event,
		rate:      normalized.Rate,
		inventory: &amount,
	})
	return queue
}

// Retrieve cost basis for sale information.
func popBasisHistory(amount data.Amount, queue heap.Interface) (inventory []TaxInventory, amountRemaining *data.Amount) {
	// Sanity
	if !amount.IsNegative() {
		log.Panicf("popBasisHistory expected negative amount, got %s", amount)
	}

	amountRemaining = amount.Clone()
	zero := amount.ZeroClone()

	for amountRemaining.IsNegative() {
		if queue == nil || queue.Len() == 0 {
			// The gain is not accounted for (i.e. a transfer from one
			// account to another).  The caller will look up the historic
			// price..
			log.Printf("Did not find historic basis for %s (of original amount %s)", amountRemaining, amount) // debug
			log.Println("Here's what was found:")
			for i, basis := range inventory {
				log.Printf("%d: %s on %s, @ %s", i, basis.inventory, basis.Transaction.GetExecutedTime().Format("2006-01-02"), basis.rate)
			}
			return
		}

		pop := heap.Pop(queue).(TaxInventory)
		var err error
		amountRemaining, err = amountRemaining.Add(pop.inventory) // negative plus positive
		if err != nil {
			log.Panic(err)
		}

		if zero.Less(*amountRemaining.Value) {
			// We popped more than enough.  Return surplus.
			copy := pop.Clone()
			copy.inventory = amountRemaining.Clone()
			heap.Push(queue, *copy)

			// Correct the amount we are about to return.
			pop.inventory, err = pop.inventory.Subtract(amountRemaining) // subtract positive number
			if err != nil {
				log.Panic(err)
			}

			amountRemaining = amountRemaining.ZeroClone()
		}

		// Return the basis we've taken off the queue.
		inventory = append(inventory, pop)
	} // end popping loop

	return
}

/*
func processBalanceChangeDescriptorXXX(record history.AccountTx, change rippledata.BalanceChangeDescriptor) error {
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
*/

// Note that "balance change objects" are not the same thing as "balance change descriptors" in data API.  Yikes!
// https://ripple.com/build/data-api-v2/#balance-objects-and-balance-change-objects
/*
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
*/

/*
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
*/

func pushCostBasis(queue heap.Interface, cost TaxInventory) heap.Interface {
	if cost.inventory.IsZero() { // sanity
		log.Panicf("Unexpected empty inventory")
	}

	if queue == nil {
		if useFIFO {
			queue = rippledata.NewTransactionFIFO()
		} else {
			queue = rippledata.NewTransactionLIFO()
		}
		heap.Init(queue)
	}

	heap.Push(queue, cost)
	return queue
}

func popCostBasis(amount data.Amount, queue heap.Interface) (basis []TaxInventory, amountRemaining *data.Amount) {
	//basis = make([]TaxInventory, 0)
	amountRemaining = amount.Clone()
	zero := amount.ZeroClone()

	// Until remainder is zero, pop from cost basis history
	for zero.Less(*amountRemaining.Value) {
		if queue == nil || queue.Len() == 0 {
			// The gain is not accounted for (i.e. a transfer from one
			// account to another).  The caller will look up the historic
			// price..
			log.Panicf("Did not find price for %s (of original amount %s)", amountRemaining, amount) // debug
			return basis, amountRemaining
		}

		pastCost := heap.Pop(queue).(TaxInventory)
		var err error
		amountRemaining, err := amountRemaining.Subtract(pastCost.inventory)
		if err != nil {
			log.Panic(err)
		}

		if amountRemaining.IsNegative() {
			// We pulled more than enough.  Put surplus back.
			copy := pastCost.Clone()
			copy.inventory = amountRemaining.Abs()
			heap.Push(queue, *copy)

			pastCost.inventory, err = pastCost.inventory.Add(amountRemaining) // add negative number
			if err != nil {
				log.Panic(err)
			}
			amountRemaining = amountRemaining.ZeroClone()
		}

		basis = append(basis, pastCost)
	}

	return basis, amountRemaining

}

// Helper to cache normalized rates.  We use the rate, not the amount, so we can get one rate per day.
var normalizedCache map[string]*rippledata.NormalizeResponse

func normalize(amount data.Amount, normalizeTo data.Asset, when time.Time) (*rippledata.NormalizeResponse, error) {
	if normalizedCache == nil {
		normalizedCache = make(map[string]*rippledata.NormalizeResponse)
	}
	key := fmt.Sprintf("%s-%s-%s", amount.Asset(), normalizeTo, when.Format("2006-01-02"))
	//log.Println("normalize key", key) // debug

	normalized, ok := normalizedCache[key]
	if !ok {
		var err error
		// Query rippledata
		normalized, err = taxDataClient.Normalize(amount, normalizeTo, when)
		if err != nil {
			return nil, err
		}
		normalizedCache[key] = normalized
	} else {
		log.Printf("Using cached normalized rate for %s on %s", amount.Currency, when.Format("2006-01-02"))
	}
	return normalized, nil
}
