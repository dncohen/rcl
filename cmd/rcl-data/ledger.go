// Copyright (C) 2019-2020  David N. Cohen
// This file is part of github.com/dncohen/rcl
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/rippledata"
	"github.com/dncohen/rcl/rippledata/history"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opLedger,
		Name:        "ledger",
		Syntax:      "ledger [-fee=false] <account> [...]",
		Description: `Operation "ledger" writes historical activity in ledger-cli format.`,
	})
}

type LedgerSplit struct {
	// Account name, typically "Assets:Crypto:RCL:something"
	Name    string
	Amount  *data.Amount
	Cost    string // a leger-cli cost (or price), i.e. "@@ 10.0 USD" or "@ 0.01 USD"
	Comment string

	event    *history.AccountTx
	suppress bool // whether to comment out this split
}

func NewLedgerSplit(event *history.AccountTx) *LedgerSplit {
	this := &LedgerSplit{
		Name:   fmt.Sprintf("Assets:Crypto:RCL:%s", formatAccount(*event.Account, nil)),
		Amount: event.Transaction.(rippledata.BalanceChangeDescriptor).GetChangeAmount(),
		event:  event,
	}
	switch t := this.event.Transaction.(type) {
	case rippledata.BalanceChangeDescriptor:
		this.Comment = t.ChangeType
	default:
		this.Comment = fmt.Sprintf("FIXME unexpected event type (%T)", t)
	}
	return this
}

func (this *LedgerSplit) String() string {
	return fmt.Sprintf("%s  %s %s ; %s", this.Name, this.Amount, this.Cost, this.Comment)
}

func (this *LedgerSplit) GetChangeAmount() *data.Amount {
	if this.Amount == nil {
		if this.event != nil {
			this.Amount = this.event.Transaction.(rippledata.BalanceChangeDescriptor).GetChangeAmount()
		}
	}
	return this.Amount
}

// More than a wrapper around BalanceChangeDescriptor.ChangeType, our
// type string adds information to help identify offsetting changes.
// Returns one of "payment debit", "payment credit", "exchange debit",
// "exchange credit"
func (this *LedgerSplit) GetChangeType() string {
	if this.event == nil {
		// synthetic
		return ""
	}
	switch t := this.event.Transaction.(type) {
	case rippledata.BalanceChangeDescriptor:
		amount := t.GetChangeAmount()
		key := t.ChangeType
		switch key {
		case "payment_source", "payment_destination":
			key = "payment"
		case "exchange", "intermediary":
			// include account in exchange, because sometimes multiple accounts can exchange during a single tx
			key = fmt.Sprintf("exchange %s", this.event.Account)
		}

		if amount.IsNegative() {
			key = key + " debit"
		} else {
			key = key + " credit"
		}
		return key
	default:
		log.Panicf("unexpected account history event type (%T)", t)
	}
	return "" // should not be reached
}

func (this *LedgerSplit) GetInvertedChangeType() string {
	key := this.GetChangeType()

	if key == "intermediary" {
		// TODO(dnc): is this correct?
		return "exchange"
	}

	if strings.HasSuffix(key, "debit") {
		return strings.Replace(key, "debit", "credit", 1)
	} else {
		return strings.Replace(key, "credit", "debit", 1)
	}
}

type LedgerTransaction struct {
	Comment []string
	Date    string
	Payee   string
	Split   []*LedgerSplit // ledger-cli "split"

	byType map[string][]int // key splits by type, array of indexes because single tx can have any number of exchanges
	offset map[int]int      // some splits offset each other

	tx   *rippledata.GetTransactionResponse
	meta *data.MetaData
}

func (this *LedgerTransaction) sanity() { // troubleshoot
	for i, _ := range this.Split {
		x, ok := this.offset[i]
		if ok && x == i {
			log.Println(this.offset)
			log.Panic("offset of self!", i)
		}
	}
}

func NewLedgerTransaction(event []*history.AccountTx) *LedgerTransaction {
	this := &LedgerTransaction{
		Split:  make([]*LedgerSplit, len(event)),
		byType: make(map[string][]int),
		offset: make(map[int]int),
	}

	this.Comment = append(this.Comment,
		fmt.Sprintf("ledger #%d | tx #%d | %s", event[0].GetLedgerIndex(), event[0].GetTransactionIndex(), event[0].GetExecutedTime()),
	)

	var offset *LedgerSplit
	for i, e := range event {
		this.Split[i] = NewLedgerSplit(e)
		typ := this.Split[i].GetChangeType()
		this.byType[typ] = append(this.byType[typ], i)

		if typ == "transaction_cost debit" {
			// add offsetting expense
			offset = &LedgerSplit{
				Name:   "Expenses:Crypto:RCL:fee",
				Amount: this.Split[i].Amount.Negate(),
			}
		}
	}
	if offset != nil {
		// fee and expense are mutually offsetting
		this.Split = append(this.Split, offset)
		feeIndex := this.byType["transaction_cost debit"][0]
		this.offset[len(this.Split)-1] = feeIndex
		this.offset[feeIndex] = len(this.Split) - 1
	}

	this.Date = this.GetExecutedTime().Format("2006-01-02")
	this.Payee = this.GetHash().String()
	command.V(2).Infof("transaction (%q) on %s has %d events", this.Payee, this.Date, len(event))

	return this
}

// SetTransaction could be called part two of constructor.  This code
// is separate from NewLedgerTransaction() because we don't always
// seek out the additional details (i.e. for fee-only balance
// changes).
func (this *LedgerTransaction) SetTransaction(tx *rippledata.GetTransactionResponse) {
	this.tx = tx
	this.meta = &tx.Transaction.Meta
	this.Payee = fmt.Sprintf("%s %s (%s)", tx.Transaction.Tx.GetType(), this.Payee, this.meta.TransactionResult)

	// type-specific comment preceeding transaction
	switch t := tx.Transaction.Tx.Transaction.(type) { // naming is hard
	case *data.Payment:
		this.Comment = append(this.Comment, fmt.Sprintf("Payment %s -> %s (%s, delivered %s)", formatAccount(t.Account, t.SourceTag), formatAccount(t.Destination, t.DestinationTag), this.meta.TransactionResult, this.meta.DeliveredAmount))

		// if events are only "exchange", we don't need to add splits for source or destination

		// if an event already for destination, add the source
		i, ok := this.byType["payment credit"]
		if ok {
			srcSplit := this.affects(this.GetBase().Account, this.GetBase().SourceTag, "tx_source")
			if srcSplit != nil {
				//log.Printf("added srcSplit, splits now %d; byType: %v; offset: %v", len(this.Split), this.byType, this.offset)
				this.offset[len(this.Split)-1] = i[0]
				this.offset[i[0]] = len(this.Split) - 1
			}
		}
		this.sanity()

		i, ok = this.byType["payment debit"]
		if ok {
			if this.meta.TransactionResult == 0 { // unfortunately tesSUCCESS not exported by rubblelabs
				dstSplit := this.affects(t.Destination, t.DestinationTag, "payment_destination") // TODO(dnc): revisit these type names
				if dstSplit != nil {
					this.offset[len(this.Split)-1] = i[0]
					this.offset[i[0]] = len(this.Split) - 1
				}
			}
		}
		this.sanity()

	default:
		this.Comment = append(this.Comment, fmt.Sprintf("%T %s (%s)", t, formatAccount(t.GetBase().Account, nil), this.meta.TransactionResult))
	}
}

func (this *LedgerTransaction) affects(account data.Account, tag *uint32, reason string) *LedgerSplit {
	// If account is included in events, no additional action required.
	for _, s := range this.Split {
		if s.event == nil {
			// here, indicates we are adding multiple synthetic splits; probably a bug
			continue
		}
		if *s.event.Account == account {
			return nil
		}
	}

	nick, ledgerAccount := accountDetail(account, tag)
	// Here, the affected account is not yet represented in a split.
	split := &LedgerSplit{
		Name:    ledgerAccount,
		Comment: reason,
	}
	if ledgerAccount == "" {
		// ledger-cli name not found in config, make one up
		split.Name = "FIXME:Crypto:RCL:" + nick
	}

	this.Split = append(this.Split, split)
	this.byType[reason] = append(this.byType[reason], len(this.Split)-1)

	return split
}

func (this *LedgerTransaction) GetHash() data.Hash256 { return this.Split[0].event.GetHash() }
func (this *LedgerTransaction) GetExecutedTime() time.Time {
	return this.Split[0].event.GetExecutedTime().In(time.Local)
}
func (this *LedgerTransaction) GetBase() *data.TxBase { return this.tx.Transaction.Tx.GetBase() }

func (this *LedgerTransaction) IdentifyOffsetCost(base data.Asset) {
	for i, s := range this.Split {
		_, ok := this.offset[i]
		if ok {
			continue
		}

		if s.event == nil {
			continue // affected (synthetic) split
		}

		switch t := s.event.Transaction.(type) {
		case rippledata.BalanceChangeDescriptor:
			amount := t.GetChangeAmount()

			if amount.Currency.String() == base.Currency {
				// associate cost with offseting split (later iteration of this loop)
				continue
			}
			if amount.IsZero() {
				continue
			}

			key := s.GetInvertedChangeType()
			offsetting, _ := this.byType[key]
			if len(offsetting) == 1 {
				offset := this.Split[offsetting[0]].GetChangeAmount()
				if offset.Currency != amount.Currency {
					// The cost of the credit, learned from offsetting debit
					if offset.IsNegative() || offset.Currency.String() == base.Currency {
						s.Cost = fmt.Sprintf("@@ %s %s", offset.Abs().Value, offset.Currency)
					} else {
						this.Split[offsetting[0]].Cost = fmt.Sprintf("@@ %s %s", amount.Abs().Value, amount.Currency)
					}
				} else {
					// same currency indicates a move from one wallet to another, no price needed
				}
				this.offset[offsetting[0]] = i
				this.offset[i] = offsetting[0]
				continue
			}
		}
	} // end each split

}

func (this *LedgerTransaction) Suppress(typ string) {
	for i, s := range this.Split {
		if strings.HasPrefix(s.GetChangeType(), typ) {
			s.suppress = true
			offset, ok := this.offset[i]
			if ok {
				// comment out offsetting tx as well (typically fee and expense)
				this.Split[offset].suppress = true
			}
		}
	}
}

func (this *LedgerTransaction) IsSuppressed() bool {
	for _, s := range this.Split {
		if !s.suppress {
			return false
		}
	}
	return true // all splits suppressed
}

func (this *LedgerTransaction) String() string {
	return fmt.Sprintf("%s %s; %s", this.Date, this.Payee, this.Comment)
}

// Formats transaction header to stdout and splits to table writer.
func (this *LedgerTransaction) RenderHead(w io.Writer) {
	fmt.Fprintln(w, "") // blank line
	for _, c := range this.Comment {
		fmt.Fprintf(w, "; %s\n", c)
	}
	fmt.Fprintf(w, "%s %s\n", this.Date, this.Payee) // payee
}

func (this *LedgerTransaction) RenderSplit(w io.Writer) {
	// two passes, put suppressed splits at the end
	for _, pass := range []bool{false, true} {
		for i, s := range this.Split {
			if s.suppress != pass {
				continue
			}
			amount := s.GetChangeAmount()
			prefix := ""
			if s.suppress {
				prefix = ";"
			}

			// suppress cost when Asset to Asset
			cost := s.Cost
			if cost != "" {
				offset, ok := this.offset[i]

				if ok && (this.Split[offset].Amount == nil || this.Split[offset].Amount.Currency == s.Amount.Currency) {
					prefix := strings.SplitN(s.Name, ":", 2)
					prefix2 := strings.SplitN(this.Split[offset].Name, ":", 2)
					if prefix[0] == prefix2[0] {
						// omit cost of like-kind offsetting splits (no gain or basis, just transfer)
						cost = ""
					}
				}
			}
			if amount != nil {
				fmt.Fprintf(w, "\t%s%s\t%s %s\t%s\t; %s", prefix, s.Name, formatValue(*amount.Value), amount.Currency, cost, s.Comment)
			} else {
				// blank split to be balanced by ledger-cli
				fmt.Fprintf(w, "\t%s%s\t   \t%s\t; %s", prefix, s.Name, cost, s.Comment)
			}
			fmt.Fprintf(w, "\n")
		}
	}
}

// cache normalized prices
var ledgerPriceCache map[data.Currency]*rippledata.NormalizeResponse
var ledgerPriceCacheExpire time.Time

func (this *LedgerTransaction) NormalizePrice(dataClient rippledata.Client, base data.Asset) map[data.Currency]*rippledata.NormalizeResponse {

	executed := this.GetExecutedTime()

	if ledgerPriceCache != nil && executed.After(ledgerPriceCacheExpire) {
		// expired outdated cache
		ledgerPriceCache = nil
	}

	if ledgerPriceCache == nil {
		ledgerPriceCache = make(map[data.Currency]*rippledata.NormalizeResponse)
		// To round to the last midnight in the local timezone, create a new Date.
		midnight := time.Date(executed.Year(), executed.Month(), executed.Day(), 0, 0, 0, 0, time.Local)
		ledgerPriceCacheExpire = midnight.Add(time.Hour * 24)
	}

	newData := make(map[data.Currency]*rippledata.NormalizeResponse)

	for i, s := range this.Split {
		if s.Cost != "" {
			// cost already known
			continue
		}

		if s.suppress { // feeFlag to suppress transaction fees
			continue
		}

		if s.event == nil { // synthetic split (via affects())
			continue
		}

		switch t := s.event.Transaction.(type) {
		case rippledata.BalanceChangeDescriptor:
			amount := t.GetChangeAmount()
			if amount.IsZero() {
				continue
			}
			if amount.Currency.String() == base.Currency {
				// no conversion needed
				continue
			}

			_, ok := ledgerPriceCache[amount.Currency]
			if !ok {
				var err error
				// make slow request for historic price
				ledgerPriceCache[amount.Currency], err = dataClient.Normalize(*amount, base, s.event.GetExecutedTime())
				if err != nil {
					command.Error("failed to normalize price of %s on %s", amount, s.event.GetExecutedTime().Format("2006/01/02 15:04:05"))
					fmt.Printf("; FIXME: failed to normalize price of %s on %s\n", amount, s.event.GetExecutedTime().Format("2006/01/02 15:04:05"))
					delete(ledgerPriceCache, amount.Currency) // just in case Normalize returned non-nil
					continue
				}
				newData[amount.Currency] = ledgerPriceCache[amount.Currency]
			}

			// omit cost if this split is already offset by another with
			// cost associated (note: adding price line to ledger-data is
			// beneficial here, so we still return price in newData)
			offset, ok := this.offset[i]
			if ok && this.Split[offset].Cost != "" {
				s.Cost = fmt.Sprintf("; @ %s %s", ledgerPriceCache[amount.Currency].Rate, base.Currency) // debug
				continue
			}

			// apply normalized price to split
			s.Cost = fmt.Sprintf("@ %s %s", ledgerPriceCache[amount.Currency].Rate, base.Currency)
		}
	}

	return newData
}

func opLedger() error {
	var defaultAsset string
	cfg, err := command.Config()
	if err != nil {
		if errors.Is(err, config.ConfigNotFound) {
			defaultAsset = "USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B" // default bitstamp
			err = nil                                              // not found is not fatal error
		}
		command.Check(err)
	} else {
		defaultAsset = cfg.Section("").Key("base").MustString("USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B") // rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B is bitstamp
	}

	// define flags
	baseFlag := command.OperationFlagSet.String("base", defaultAsset, "query for price relative to base")
	feeFlag := command.OperationFlagSet.Bool("fee", false, "include transaction fees")
	endFlag := command.OperationFlagSet.String("end", "", "last date to include")
	nFlag := command.OperationFlagSet.Int("n", 0, "how many transactions to inspect (for debugging); use 0 for all")

	allFlag := command.OperationFlagSet.Bool("all", false, "query status of transaction, even if balance unaffected")

	// parse flags
	err = command.ParseOperationFlagSet()
	if err != nil {
		return err
	}

	// validate flags
	if len(command.OperationFlagSet.Args()) == 0 {
		return errors.New("Expected <account> parameter.")
	}
	account, err := parseAccountArg(command.OperationFlagSet.Args())
	command.Check(err)

	namedAccount := make(map[string]*data.Account) // our balance change iterator needs this TODO(dnc) still needed?
	for i, arg := range command.OperationFlagSet.Args() {
		namedAccount[arg] = &(account[i].Account)
	}

	var base *data.Asset
	if *baseFlag != "" {
		base, err = data.NewAsset(*baseFlag)
		command.Check(err)
	}

	var endDate time.Time
	if *endFlag != "" {
		endDate, err = time.Parse("2006-01-02", *endFlag)
		command.Check(err)
		// To round to the last midnight in the local timezone, create a new Date.
		endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.Local)
		log.Println("endDate:", endDate)
	}

	dataAPI, err := cmd.DataAPI()
	command.Check(err)
	dataClient, err := rippledata.NewClient(dataAPI)
	command.Check(err)

	command.V(1).Infof("Inspecting %d account(s) via %q", len(account), dataAPI)

	fmt.Printf("; rcl-data ledger -fee=%t -base=%q -n=%d %s\n", *feeFlag, *baseFlag, *nFlag, strings.Join(command.OperationFlagSet.Args(), " "))

	// Iterate over balance changes for multiple accounts, in global chronological order
	var event []*history.AccountTx
	iterator := history.NewBalanceChangeIterator(dataClient, namedAccount)
	err = iterator.Init()
	command.Check(err)

	// iterator determines when history starts for each account
	for nick, data := range iterator.AccountData {
		command.V(1).Infof("%s created by %s at %s", nick, formatAccount(data.Parent, nil), data.Inception)
		fmt.Printf("; %s created by %s at %s\n", nick, formatAccount(data.Parent, nil), data.Inception)
	}

	// tabwriter to format ledger-cli splits
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	txCount := 0
	// loop over all past transactions
	for event = iterator.Next(); event != nil; event = iterator.Next() {
		txCount++
		if *nFlag > 0 && txCount > *nFlag {
			command.Infof("exiting after %d transactions (-n flag)", txCount-1)
			fmt.Printf("; rcl-data ledger: exit after %d transactions (-n=%d flag )\n", txCount-1, *nFlag)
			break
		}

		// build ledger-cli transaction, where each ripple-data "event" is a "split"
		ledgerTx := NewLedgerTransaction(event)
		ledgerTx.sanity()

		if *endFlag != "" && endDate.Before(ledgerTx.GetExecutedTime()) {
			command.Infof("reached end date %q (before transaction %q)", *endFlag, ledgerTx)
			fmt.Printf("; reached end date %q (-end flag)", *endFlag)
			break
		}

		if !*feeFlag {
			ledgerTx.Suppress("transaction_cost")
		}

		if !ledgerTx.IsSuppressed() || *allFlag {

			// Query data api for the transaction responsible for balance
			// changes.  This allows us to learn additional details not
			// available from balance change events (i.e. the sender of a
			// payment we received)
			txHash := event[0].GetHash()
			tx, err := dataClient.Transaction(txHash) // TODO(dnc) concurrent requests
			if err != nil {
				command.Error(err)
			}
			ledgerTx.SetTransaction(tx)
			ledgerTx.sanity()

			if base != nil {
				// track offsetting costs
				ledgerTx.IdentifyOffsetCost(*base)

				normalized := ledgerTx.NormalizePrice(dataClient, *base)

				// write price history in ledger-cli format
				for currency, norm := range normalized {
					fmt.Printf("\n; Value of %s %s is %s %s on %s\n", norm.Amount, currency, norm.Converted, base.Currency, ledgerTx.GetExecutedTime().Format("2006/01/02 15:04:05 (MST)"))
					// https://www.ledger-cli.org/3.0/doc/ledger3.html#Commodity-price-histories
					fmt.Printf("P %s %s %s %s\n",
						ledgerTx.GetExecutedTime().Format("2006/01/02 15:04:05"),
						currency,
						norm.Rate, base.Currency,
					)
				}
			}
		} // end if suppressed

		ledgerTx.RenderHead(os.Stdout)
		ledgerTx.RenderSplit(writer)

		writer.Flush()
		fmt.Println()
	} // end event iterator loop

	return nil
}
