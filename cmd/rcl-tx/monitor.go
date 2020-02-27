// Copyright (C) 2018-2020  David N. Cohen
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

// Operation monitor
//
// Monitor RCL for transaction activity.
//
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/util"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/websockets"
	"github.com/y0ssar1an/q"
	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opMonitor,
		Name:        "monitor",
		Syntax:      "monitor ",
		Description: `Monitor RCL for transaction activity.`,
	})
}

func opMonitor() error {

	sinceFlag := command.OperationFlagSet.Int("since", 0, "ledger sequence number where monitoring will start; use 0 for most recent.")

	command.CheckUsage(command.ParseOperationFlagSet())

	rippled, err := cmd.Rippled()
	command.Check(err)

	// A subscription lets us know when new ledgers are validated.
	subscription, err := util.NewSubscription(rippled)
	command.Check(err)

	go subscription.Loop()
	command.V(1).Infof("connected to %q", rippled)

	min, max, err := subscription.Ledgers()
	if err != nil {
		command.Check(fmt.Errorf("failed to get available ledgers from %q: %w", rippled, err))
	}
	command.V(1).Infof("%s ledger history %d - %d\n", rippled, min, max)

	since := uint32(*sinceFlag)
	if since == 0 {
		since = max
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
				command.Check(fmt.Errorf("failed to get ledger %d, available history is %d-%d.", idx, min, max))
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

			//log.Printf("Requesting ledger %d transactions...", idx) // verbose
			ledgerResult, err := subscription.Remote.Ledger(idx, true)
			q.Q(ledgerResult) // troubleshooting
			os.Exit(2)        // XXX work in progress
			if err != nil {
				log.Println(errors.Wrapf(err, "Error requesting transactions in ledger %d.\n", idx))
				// Put the same ledger index back on the queue, so we try again later.
				go func(idx uint32) {
					ledgerIndexes <- idx
				}(idx)
				continue
			}

			// Scan for transactions we're interested in.
			for _, tx := range ledgerResult.Ledger.Transactions {
				_, ok := cmd.AccountConfig(tx.GetBase().Account, tx.GetBase().SourceTag)
				if !ok {
					//log.Printf("Nickname not found for %s\n", tx.GetBase().Account)
					// Only show transactions affecting account in our config file.
					continue
				}

				nick := cmd.FormatAccount(tx.GetBase().Account, tx.GetBase().SourceTag)

				// TODO are all tx validated?

				// Note these tx lack metadata! TODO, use account_tx or subscribe to transaction fee and inspect affected nodes.
				// With data returned by Ledger we can filter by Account signer, but not affected nodes.
				// TODO: Why is ledger always 0? <- because metadata all empty.

				command.Infof("%s - %s, %s (%s) sequence %d, %s in ledger %d",
					tx.GetHash(), tx.GetType(), tx.GetBase().Account, nick, tx.GetBase().Sequence, tx.MetaData.TransactionResult, idx /*tx.LedgerSequence*/)

				// TODO: was a marker returned, do we need to paginate?
			}
			// Add the next ledger sequence to our queue.
			go func(ledgerResult *websockets.LedgerResult) {
				ledgerIndexes <- ledgerResult.Ledger.LedgerHeader.LedgerSequence + 1
			}(ledgerResult)
		}
	}

	// TODO(dnc): clean up on interrupt

	//return nil
}
