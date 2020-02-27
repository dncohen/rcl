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

// Operation trust
//
// Create or modify a trust line.
//
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/internal/pipeline"
	"github.com/dncohen/rcl/tx"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opTrust,
		Name:        "trust",
		Syntax:      "trust [ripple=<true or false>] [authorize=<true or false>] <amount>",
		Description: `Create or modify a trust line.`,
	})
}

func opTrust() error {

	rippleFlag := command.OperationFlagSet.Bool("ripple", false, "allow rippling on account side of trustline, if true; disallow if false")
	authorizeFlag := command.OperationFlagSet.Bool("authorize", false, "if true, set authorize flag on account side of trustline; if false, make no change to authorization flag")

	// TODO quality

	command.CheckUsage(command.ParseOperationFlagSet())

	// TODO!
	if *rippleFlag || *authorizeFlag {
		command.Check(errors.New("ripple flag and authorize flag not yet supported (sorry)"))
	}

	argument := command.OperationFlagSet.Args()
	if len(argument) != 1 {
		// TODO(dnc): is this required when authorizing?
		command.CheckUsage(errors.New("operation requires amount of trust line"))
	}
	fail := false

	amount, err := cmd.AmountFromArg(argument[0])
	if err != nil {
		command.Errorf("bad amount (%q): %w", argument[0], err)
		fail = true
	} else if amount.IsNative() {
		command.Errorf("bad amount (%q): cannot set trust for XRP", amount)
		fail = true
	}

	if asAccount == nil {
		fail = true
		command.Errorf("operation requires -as <address> flag")
	}

	if fail {
		command.Exit()
	}

	// TODO confirm
	command.Infof("Set trust %s ---> %s\n", asAccount, amount)

	rippled, err := cmd.Rippled()
	command.Check(err)

	// Learn the needed detail of the account setting the line.
	remote, err := websockets.NewRemote(rippled)
	command.Check(fmt.Errorf("failed to connect to %q: %w", rippled, remote))
	defer remote.Close()

	command.Infof("connected to %q", rippled) // verbose

	// Interestingly, rubblelabs commands do not support server_info!
	// As it happens, the account_info returns ledger_current_index,
	// which allows us to compute a LastLedgerSequence.

	// Here we also call for fee... but don't actually use the result.

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult

	g.Go(func() error {
		var err error
		accountInfo, err = remote.AccountInfo(*asAccount)
		if err != nil {
			return fmt.Errorf("failed to get account_info %s: %w", asAccount, err)
		}
		return nil
	})

	// not currently using...we could omit this.
	/*
		var feeInfo *websockets.FeeResult
			g.Go(func() error {
				var err error
				feeInfo, err = remote.Fee()
				if err != nil {
					log.Printf("Failed to get fee: %s", err)
					return err
				}
				return nil
			})
	*/
	err = g.Wait()
	command.Check(err)

	// Prepare to encode transaction output.
	unsignedOut := make(chan (data.Transaction))
	g.Go(func() error {
		return pipeline.EncodeOutput(os.Stdout, unsignedOut)
	})

	// Prepare a TrustSet transaction.
	t, err := tx.NewTrustSet(
		tx.SetAddress(asAccount),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12),
		tx.SetLimitAmount(*amount),
		// TODO flags
		// TODO qualityin, qualityout

		tx.AddMemo(memoFlag), // TODO support multiple memo fields
		tx.AddMemo(memohex),

		tx.SetCanonicalSig(true),
	)

	// TODO: is it necessary to clean up the hash that rubblelabs puts into unsigned tx?
	// "hash":"0000000000000000000000000000000000000000000000000000000000000000"

	// Show in json format (debug)
	j, _ := json.MarshalIndent(t, "", "\t")
	log.Printf("Unsigned:\n%s\n", string(j))
	// In case user in on a terminal, nice to have a clean line.
	fmt.Fprintf(os.Stderr, "\n")

	// marshall the tx to stdout pipeline
	unsignedOut <- t
	close(unsignedOut)

	// Wait for all output to be encoded
	err = g.Wait()
	command.Check(err)

	return nil

}
