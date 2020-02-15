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

// Operation send
//
// Send XRP or issuance.
//
package main

import (
	"fmt"
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

var (
	zeroAccount data.Account // TODO(dnc): move to internal utils
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opSend,
		Name:        "send",
		Syntax:      "send  <beneficiary> <amount>",
		Description: `Send an RCL asset or issuance from one account to another.`,
	})
}

func opSend() error {

	sendmaxFlag := command.OperationFlagSet.String("sendmax", "", "Specify SendMax, allows cross-currency payment")

	command.CheckUsage(command.ParseOperationFlagSet())

	fail := false

	// command line args
	var sendMax *data.Amount

	if *sendmaxFlag != "" {
		var err error
		sendMax, err = data.NewAmount(*sendmaxFlag)
		if err != nil {
			command.Check(fmt.Errorf("bad sendmax (%q): %w", *sendmaxFlag, err))
		}
	}

	argument := command.OperationFlagSet.Args()
	if len(argument) != 2 {
		command.CheckUsage(errors.New("operation requires <destination> and <amount> arguments"))
	}

	beneficiaryArg, err := cmd.ParseAccountArg(argument[0:1])
	if err != nil {
		command.Errorf("bad beneficiary address (%q): %s", argument[0], err)
		fail = true
	}
	beneficiary := beneficiaryArg[0].Account
	beneficiaryTag := &beneficiaryArg[0].Tag
	if *beneficiaryTag == 0 {
		beneficiaryTag = nil
	}

	amount, err := cmd.AmountFromArg(argument[1])
	if err != nil {
		command.Errorf("bad amount (%q): %s", argument[1], err)
		fail = true
	}

	rippled, err := cmd.Rippled()
	if err != nil {
		command.Errorf(err.Error())
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

	// Connect, to learn LastLedgerSequence and account sequence.
	remote, err := websockets.NewRemote(rippled)
	command.Check(err)

	// TODO Want to close, but leads to "use of closed network connection" error.
	//defer remote.Close()

	// Use an errgroup in case we eventually need multiple calls, i.e. to get fee information.
	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult
	g.Go(func() error {
		var err error
		accountInfo, err = remote.AccountInfo(*asAccount)
		if err != nil {
			return fmt.Errorf("failed to get account_info (%s): %w", asAccount, err)
		}
		return nil
	})
	err = g.Wait()
	command.Check(err)

	// Ensure no ambiguity in amounts or issuers.
	if !amount.IsNative() && amount.Issuer == zeroAccount {
		command.V(1).Infof("using %s as %s issuer", beneficiary, amount.Currency)
		amount.Issuer = beneficiary
	}
	if sendMax == nil && !amount.IsNative() { // No sendmax on XRP payments
		sendMax = amount
	}
	if sendMax != nil && !sendMax.IsNative() && sendMax.Issuer == zeroAccount {
		sendMax.Issuer = *asAccount
	}

	tx, err := tx.NewPayment(
		tx.SetAddress(asAccount),
		tx.SetSourceTag(asTag),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12), // TODO

		tx.AddMemo(memoFlag), // TODO support multiple memo fields
		tx.AddMemo(memohex),

		// Simple payment, source and destination currency the same.
		tx.SetAmount(amount),
		tx.SetSendMax(sendMax),

		tx.SetDestination(beneficiary),
		tx.SetDestinationTag(beneficiaryTag),

		tx.SetCanonicalSig(true),
	)

	// Prepare to encode transaction output.
	unsignedOut := make(chan (data.Transaction))
	g.Go(func() error {
		return pipeline.EncodeOutput(os.Stdout, unsignedOut)
	})

	// Pass unsigned transaction to encoder
	unsignedOut <- tx
	close(unsignedOut)

	err = g.Wait()
	command.Check(err)

	command.V(1).Infof("Prepared unsigned %s from %s to %s.\n", tx.GetType(), tx.Account, tx.Destination)

	return nil
}
