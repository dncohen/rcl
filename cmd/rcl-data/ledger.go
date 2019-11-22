// Copyright (C) 2019  David N. Cohen
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
	"os"
	"text/tabwriter"

	"github.com/dncohen/rcl/rippledata"
	"github.com/dncohen/rcl/rippledata/history"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     ledgerMain,
		Name:        "ledger",
		Syntax:      "ledger <account> [...]",
		Description: `Operation "ledger" writes historical activity in ledger-cli format.`,
	})
}

func ledgerMain() error {

	nFlag := command.OperationFlagSet.Int("n", 0, "how many transactions to inspect (for debugging); use 0 for all")

	err := command.OperationFlagSet.Parse(command.Args()[1:])
	if err != nil {
		return err
	}

	if len(command.OperationFlagSet.Args()) == 0 {
		return errors.New("Expected <account> parameter.")
	}
	account, err := parseAccountArg(command.OperationFlagSet.Args())
	command.Check(err)
	namedAccount := make(map[string]*data.Account) // our balance change iterator needs this
	for _, a := range account {
		tmp := a
		namedAccount[formatAccount(a)] = &tmp
	}

	// TODO make base asset configurable
	base, err := data.NewAmount("1.0/USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B") // rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B is bitstamp
	command.Check(err)

	// TODO(dnc): make data API url configurable
	dataAPI := "https://data.ripple.com/v2/" // trailing slash needed.
	dataClient, err := rippledata.NewClient(dataAPI)
	command.Check(err)

	command.V(1).Infof("Inspecting %d account(s) via %q", len(account), dataAPI)

	// Iterate over balance changes for each account
	var event []*history.AccountTx
	iterator := history.NewBalanceChangeIterator(dataClient, namedAccount)
	err = iterator.Init()
	command.Check(err)

	if command.V(1) {
		for nick, data := range iterator.AccountData {
			command.Infof("%s created by %s at %s", nick, formatAccount(data.Parent), data.Inception)
		}
	}

	// tabwriter to format ledger-cli splits
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	// prepare to loop over all past transactions
	count := 0

	for event = iterator.Next(); event != nil; event = iterator.Next() {
		count++
		if *nFlag > 0 && count > *nFlag {
			command.Infof("aborting after %d transactions (-n flag)", count-1)
			break
		}
		// note, iterator may return one or more events per transaction
		txDate := event[0].GetExecutedTime().Format("2006-01-02")
		txHash := event[0].GetHash()
		command.V(2).Infof("transaction (%q) has %d events", txHash, len(event))

		tx, err := dataClient.Transaction(txHash)
		if err != nil {
			command.Error(err)
		}
		//q.Q(tx) // debug
		txMeta := tx.Transaction.Meta

		// track accounts known to be affected by the transaction
		affected := make(map[data.Account]string)
		affected[tx.Transaction.Tx.GetBase().Account] = "tx source"

		// type-specific comment preceeding transaction
		switch t := tx.Transaction.Tx.Transaction.(type) { // naming is hard
		case *data.Payment:
			fmt.Printf("\n; Payment %s -> %s (%s, delivered %s)\n", formatAccount(t.Account), formatAccount(t.Destination), txMeta.TransactionResult, txMeta.DeliveredAmount)
			if txMeta.TransactionResult == 0 { // unfortunately tesSUCCESS not exported by rubblelabs
				affected[t.Destination] = "payment_destination"
			}
		default:
			fmt.Printf("\n; %T %s (%s)\n", t, formatAccount(t.GetBase().Account), txMeta.TransactionResult)
		}
		// new ledger-cli transaction starts payee line
		fmt.Printf("%s %s %s (%s)\n", txDate, tx.Transaction.Tx.GetType(), txHash, txMeta.TransactionResult) // payee

		// track which accounts are shown in splits
		shown := make(map[data.Account]bool)

		for _, e := range event {
			switch t := e.Transaction.(type) {
			case rippledata.BalanceChangeDescriptor:
				amount := t.GetChangeAmount()
				//counterparty := formatAccount(t.Counterparty)
				fmt.Fprintf(writer, "\tAsset:Crypto:RCL:%s\t%s %s\t; %s\n", formatAccount(*e.Account), amount.Value, amount.Currency, t.ChangeType) // split
				shown[*e.Account] = true

				if t.ChangeType == "transaction_cost" {
					// add split for fees
					fmt.Fprintf(writer, "\tExpense:Crypto:RCL:fee\t%s %s\t; %s\n", amount.Value.Negate(), amount.Currency, t.ChangeType)
				}
			default:
				command.Errorf("Unexpected event type (%T)", t)
			}
		}

		// when an account is known to be affected, but not shown, add a blank split which human may be able to better classify
		for a, comment := range affected {
			isShown, _ := shown[a]
			if !isShown {
				fmt.Fprintf(writer, "\tFIXME:Crypto:RCL:%s\t \t; %s\n", formatAccount(a), comment)
				shown[a] = true
			}
		}

		writer.Flush()
		fmt.Println()
		_ = txDate
	}

	_ = account
	_ = base
	return nil
}
