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

// Operation submit
//
// Submit command broadcasts signed transactions to a rippled server.
//
package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/util"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/rubblelabs/ripple/data"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opSubmit,
		Name:        "submit",
		Syntax:      "submit ",
		Description: `Submit a signed transaction to a rippled server.`,
	})
}

func opSubmit() error {

	command.CheckUsage(command.ParseOperationFlagSet())

	// TODO(dnc): support files or pipelin

	// Read incoming signed transactions from stdin
	signedTransactions := make(chan (data.Transaction))
	go func() {
		err := marshal.DecodeTransactions(os.Stdin, signedTransactions)
		if err != nil {
			if err == io.EOF {
				// Expected at end of input
				// TODO: ensure there's been at least one
			} else {
				command.Check(err)
			}
			close(signedTransactions)
		}
	}()

	rippled, err := cmd.Rippled()
	command.Check(err)

	// Subscription makes it easier to wait for transaction validation.
	//log.Printf("Connecting to %s...\n", rippled) // debug
	subscription, err := util.NewSubscription(rippled)
	if err != nil {
		command.Check(fmt.Errorf("failed to connect to %q: %w", rippled, err))
	}
	go subscription.Loop()
	command.V(1).Infof("connected to %q", rippled) // verbose

	// errgroup in order to wait for results of all submitted tx.
	var g errgroup.Group

	// Submit all transaction decoded
	for tx := range signedTransactions {
		tx := tx // scope (needed ?)

		// TODO(dnc): support interactive mode where user is prompted before submitting.

		g.Go(func() error {
			tx := tx // Scope (needed?)
			result, err := subscription.SubmitWait(tx)
			if err != nil {
				return fmt.Errorf("failed to submit %s (%s): %w", tx.GetType(), tx.GetHash(), err)
			}

			if !result.Validated {
				return fmt.Errorf("%s transaction %s (tentative %s): failed to validate", tx.GetType(), tx.GetHash(), result.MetaData.TransactionResult)
			} else {
				// Show result of validated transaction.
				msg := fmt.Sprintf("%s %s (%s/%d) %s in ledger %d.\n", tx.GetType(), tx.GetHash(), tx.GetBase().Account, tx.GetBase().Sequence, result.MetaData.TransactionResult, result.LedgerSequence)
				if result.MetaData.TransactionResult.Success() {
					command.Info(msg)
				} else {
					command.Error(msg)
				}
				fmt.Println(msg) // stdout
			}

			return err
		})
	}

	// Wait for result of each submitted transaction.
	err = g.Wait()
	command.Check(err)

	return nil
}
