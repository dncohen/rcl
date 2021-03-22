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

// Operation sell
//
// Create an offer to sell one asset or issuance for another.
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
		Handler:     opSell,
		Name:        "sell",
		Syntax:      "sell <amount> for <amount>",
		Description: `Create an offer to sell one asset or issuance for another.`,
	})
}

func opSell() error {

	command.CheckUsage(command.ParseOperationFlagSet())

	argument := command.OperationFlagSet.Args()

	if len(argument) < 3 {
		command.CheckUsage(errors.New("expected arguments: <amount-to-sell> for <amount-to-accrue>"))
	}

	fail := false

	takerGets, err := data.NewAmount(argument[0])
	if err != nil {
		command.Errorf("bad amount to sell (%q): %s", argument[0], err)
		fail = true
	}

	takerPays, err := data.NewAmount(argument[2])
	if err != nil {
		command.Errorf("bad amount to accrue (%q): %s", argument[2], err)
		fail = true
	}

	// -as <account> is parsed in main.go
	if asAccount == nil {
		command.Errorf("operation requires -as <account> flag")
		fail = true
	}

	if fail {
		command.Exit()
	}

	// Make the user type "for", less likely to mistakenly reverse the amounts.
	if argument[1] != "for" {
		command.Check(errors.New("Expected `sell <amount> for <amount>`."))
	}

	command.Infof("sell %s from %s in exchange for %s...\n", takerGets, asAccount, takerPays)

	rippled, err := cmd.Rippled()
	command.Check(err)

	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		command.Check(fmt.Errorf("Failed to connect to %q: %w", rippled, err))
	}
	defer remote.Close()

	command.V(1).Infof("Connected to %q", rippled)

	// account_info returns ledger_current_index,
	// which allows us to compute a LastLedgerSequence.

	// Here we also call for fee... but don't yet use the result.

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult
	var bookOffers *websockets.BookOffersResult
	//var feeInfo *websockets.FeeResult

	g.Go(func() error {
		var err error
		accountInfo, err = remote.AccountInfo(*asAccount)
		if err != nil {
			command.Errorf("failed to get account_info %s: %s", asAccount, err)
			return err
		}
		return nil
	})

	g.Go(func() error {
		var err error
		bookOffers, err = remote.BookOffers(*asAccount, "validated", *takerPays.Asset(), *takerGets.Asset())
		if err != nil {
			command.Errorf("Failed to get book_offers: %s", err)
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

	command.V(1).Infof("order book has %d offers", len(bookOffers.Offers))

	if len(bookOffers.Offers) > 0 {
		// This is work in progress... to detect whether our order is far off the current order book.  TODO
		top := bookOffers.Offers[0]
		log.Printf("top quality: %s", top.Quality)
		log.Printf("top taker pays %s, gets %s", top.TakerPays, top.TakerGets)
		topRatio := top.TakerPays.Ratio(*top.TakerGets)
		log.Printf("book ratio: %s\n", topRatio)

		ourRatio := takerPays.Ratio(*takerGets)
		log.Printf("Our offer ratio: %s", ourRatio)

		ratRatio, err := topRatio.Ratio(*ourRatio)
		command.Check(err)
		log.Printf("ratio of ratios: %s", ratRatio)

		// TODO: inspect other side of order book and determine if offer will cross.
		if ratRatio.Compare(one) > 0 {
			command.Infof("placing crossing offer into order book")
		}

	}

	// Prepare to encode transaction output.
	unsignedOut := make(chan (data.Transaction))
	g.Go(func() error {
		return pipeline.EncodeOutput(os.Stdout, unsignedOut)
	})

	// Prepare transaction.
	offer, err := tx.NewOfferCreate(
		tx.SetAddress(asAccount),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12),

		tx.SetTakerPays(takerPays),
		tx.SetTakerGets(takerGets),
		tx.SetFlags(data.TxSell),

		tx.SetCanonicalSig(true),
	)

	// TODO: is it necessary to clean up the hash that rubblelabs puts into unsigned tx?
	// "hash":"0000000000000000000000000000000000000000000000000000000000000000"

	// Show in json format (debug)
	j, _ := json.MarshalIndent(offer, "", "\t")
	log.Printf("Unsigned:\n%s\n", string(j))

	// Pass unsigned transaction to encoder
	unsignedOut <- offer
	close(unsignedOut)

	err = g.Wait()
	command.Check(err)

	// In case user in on a terminal, nice to have a clean line.
	fmt.Fprintf(os.Stderr, "\n")
	command.Infof("unsigned %s by %s prepared", offer.GetType(), offer.GetBase().Account)

	return nil
}
