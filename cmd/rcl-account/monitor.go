// Copyright (C) 2018-2020  David N. Cohen
// This file is part of github.com/dncohen/rcl
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Command RCL-account - Operation Monitor
//
//     rcl-account monitor <address>
//
// Shows account activity as soon as it is detected.
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/util"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opMonitor,
		Name:        "monitor",
		Syntax:      "monitor [-since=<int>] <account> [...]",
		Description: `Monitor RCL for activity related to an account.`,
	})
}

func opMonitor() error {

	// subcommand-specific flags
	sinceFlag := command.OperationFlagSet.Int("since", -1, "show activity following a specific ledger; use -1 for most recent")

	err := command.ParseOperationFlagSet()
	command.CheckUsage(err)

	rippled, err := cmd.Rippled()
	command.Check(err)

	account, err := cmd.ParseAccountArg(command.OperationFlagSet.Args())
	command.Check(err)

	if len(account) == 0 {
		command.CheckUsage(errors.New("expected one or more addresses"))
	}
	command.Infof("Monitoring %d accounts", len(account))

	// A subscription lets us know when new ledgers are validated.
	subscription, err := util.NewSubscription(rippled)
	if err != nil {
		command.Check(fmt.Errorf("Failed to connect to %q: %w", rippled, err))
	}

	go subscription.Loop()
	command.V(1).Infof("connected to %q", rippled)

	min, max, err := subscription.Ledgers()
	if err != nil {
		command.Check(fmt.Errorf("failed to get available ledgers from %q: %w", rippled, err))
	}
	command.V(1).Infof("%s ledger history %d - %d\n", rippled, min, max)

	var since uint32
	switch *sinceFlag {
	case -1:
		since = min
	case 0:
		since = max
	default:
		since = uint32(*sinceFlag)
	}

	if since < min || since > max {
		command.Check(fmt.Errorf("Cannot start with ledger %d.  History available on %s is %d-%d.\n", since, rippled, min, max))
	}

	// Scan ledger indexes one by one, so as never to miss data.  We
	// want to inspect transactions in the order they occur (even if our
	// service is offline from time to time).  Use a channel to queue
	// which ledger instances need to be inspected.  Prepare it with the
	// first sequence to scan.
	ledgerIndexes := make(chan uint32, 1)
	ledgerIndexes <- since

	// Loop indefinitely
	for {

		select {

		case idx := <-ledgerIndexes:

			// Wait for the ledger sequence, if necessary.
			min, max, err := subscription.Ledgers()
			if err != nil {
				command.Infof("Failed query ledger history: %s\n", err)
				// try again.  TODO sleep first?
				go func(idx uint32) {
					ledgerIndexes <- idx
				}(idx)
				continue
			}

			if min > idx {
				command.Check(fmt.Errorf("Failed to get ledger %d, available history is %d-%d.", idx, min, max))
			}
			if max < idx {
				//log.Printf("Waiting for ledger %d...\n", idx)
				seq := <-subscription.AfterSequence(idx)
				// log.Printf("...ledger %d now available.\n", seq)
				// Sanity check
				if seq != idx {
					command.Check(fmt.Errorf("Unexpected %d returned from subscription.AfterSequence(%d).", seq, idx))
				}
			}

			//log.Printf("calling account_tx for ledger %d\n", idx) // debug verbose

			// Map used to order transactions within the specific ledger.
			txs := make(map[uint32]*data.TransactionWithMetaData)
			_ = txs
			g := new(errgroup.Group)
			for _, acct := range account {
				g.Go(func() error {
					//log.Printf("requesting %d", idx) // debug
					txChan := subscription.Remote.AccountTx(acct.Account, 10, int64(idx), int64(idx))
					for tx := range txChan {
						// transactions will be shown in order they are applied to ledger.
						txs[tx.MetaData.TransactionIndex] = tx
					}
					return nil
				})
			}
			// Wait for all account_tx calls to return
			err = g.Wait()
			if err != nil {
				// Not sure whether to retry here?
				log.Printf("Failed to get tx ledger %d: %s\n", idx, err)
				// Put the same ledger index back on the queue, so we try again later.
				go func(idx uint32) {
					ledgerIndexes <- idx
				}(idx)
				continue
			}

			if len(txs) > 0 {
				// Render each ledger as a table.
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns) // last parameter flags could include tabwriter.Debug
				fmt.Fprintln(w, util.FormatTransactionWithMetaDataHeader())

				// Order output by order within ledger
				order := make([]int, 0)
				for ledgerOrder, _ := range txs {
					order = append(order, int(ledgerOrder))
				}
				sort.Ints(order)
				for _, ledgerOrder := range order {
					tx := txs[uint32(ledgerOrder)]
					fmt.Fprintln(w, util.FormatTransactionWithMetaDataRow(tx))
					//log.Printf("%s - %s, %s sequence %d, %s in ledger %d (%d)",
					//tx.GetHash(), tx.GetType(), tx.GetBase().Account, tx.GetBase().Sequence, tx.MetaData.TransactionResult, idx, ledgerOrder)

					// Show verbose description
					for _, lint := range util.LintTransaction(tx) {
						fmt.Fprintln(w, lint) // ruins table?
					}
				}
				w.Flush()

				//s.ExitNow() // debug
			}

			// Add the next ledger sequence to our queue.
			go func(i uint32) {
				ledgerIndexes <- i + 1
			}(idx)
		}
	}

}
