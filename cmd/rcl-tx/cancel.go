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

// Opeation cancel
//
// Compose an RCL transaction to cancel an earlier offer.
//
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/tx"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opCancel,
		Name:        "cancel",
		Syntax:      "cancel [-all] [<sequence> ...]",
		Description: `Cancel an offer to sell one asset or issuance for another.`,
	})
}

func opCancel() error {

	allFlag := command.OperationFlagSet.Bool("all", false, "Cancel all outstanding offers for an account.")

	// parse flags
	err := command.ParseOperationFlagSet()
	if err != nil {
		return err
	}

	argument := command.OperationFlagSet.Args()

	if len(argument) == 0 && !*allFlag {
		return errors.New("expected sequence number of offer to cancel (or -all flag)")
	}

	if *asFlag == "" {
		return errors.New("use -as <address> flag to specify an account.")
	}

	fail := false

	seqs := make([]uint32, 0)
	for _, arg := range argument {
		i, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Printf("Expected account sequence number, got %s: %s", arg, err)
			fail = true
		} else if i < 1 {
			fmt.Printf("Expected account sequence number, got %d (less than 1)", i)
			fail = true
		}
		seqs = append(seqs, uint32(i))
	}

	if fail {
		command.Check(errors.New("correct errors and try again"))
	}

	rippled, err := cmd.Rippled()
	command.Check(err)

	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		command.Check(fmt.Errorf("failed to connect to %q: %w", rippled, err))
	}
	defer remote.Close()

	command.V(1).Infof("Connected to %s\n", rippled) // debug

	if *allFlag {
		// Cancel all of an account's outstanding offers.
		result, err := remote.AccountOffers(*asAccount, "current")
		if err != nil {
			command.Check(fmt.Errorf("account_offers failed for %s: %w", asAccount, err))
		}
		for _, offer := range result.Offers {
			seqs = append(seqs, offer.Sequence)
		}
	}

	if len(seqs) < 1 {
		command.Check(errors.New("No offers - nothing to do."))
	}

	command.Infof("Cancel %d offer(s) by %s...\n", len(seqs), asAccount)

	// account_info returns ledger_current_index,
	// which allows us to compute a LastLedgerSequence.

	// Here we also call for fee... but don't yet use the result.

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult
	//var feeInfo *websockets.FeeResult

	g.Go(func() error {
		var err error
		accountInfo, err = remote.AccountInfo(*asAccount)
		if err != nil {
			command.Errorf("Failed to get account_info %s: %s", asAccount, err)
			return err
		}
		return nil
	})

	// not currently using...we could omit this.
	/*
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
	txs := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txs)
	})

	// Prepare transactions.
	sequence := *accountInfo.AccountData.Sequence
	for _, offerSeq := range seqs {
		t, err := tx.NewOfferCancel(
			tx.SetAddress(asAccount),
			tx.SetSequence(sequence),
			tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
			tx.SetFee(12),
			tx.SetOfferSequence(offerSeq),
			tx.SetCanonicalSig(true),
		)
		if err != nil {
			command.Check(fmt.Errorf("failed to prepare OfferCancel: %w", err))
		}
		sequence++
		// TODO: is it necessary to clean up the hash that rubblelabs puts into unsigned tx?
		// "hash":"0000000000000000000000000000000000000000000000000000000000000000"

		// Show in json format (debug)
		j, _ := json.MarshalIndent(t, "", "\t")
		log.Printf("Unsigned:\n%s\n", string(j))
		// In case user in on a terminal, nice to have a clean line.
		fmt.Fprintf(os.Stderr, "\n")

		log.Printf("Unsigned %s by %s prepared.\n", t.GetType(), t.GetBase().Account)

		// Pass unsigned transaction to encoder
		txs <- t
	}
	close(txs)

	// Wait for all output to be encoded
	err = g.Wait()
	command.Check(err)

	return nil

}
