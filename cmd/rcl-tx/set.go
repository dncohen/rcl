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

// Operation set
//
// Compose an RCL transaction to change account settings.
//
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/dncohen/rcl/tx"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

const (
	unchanged = "UNCHANGED"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opSet,
		Name:        "set",
		Syntax:      "set ",
		Description: `Set a flag or field on an RCL account.`,
	})
}

func opSet() error {

	domainFlag := command.OperationFlagSet.String("domain", unchanged, "The domain that owns this account, in lower case.")

	command.CheckUsage(command.ParseOperationFlagSet())

	if *asFlag == "" {
		return errors.New("operation requires -as <account> flag")
	}

	if *domainFlag != unchanged {
		domainLower := strings.ToLower(*domainFlag)
		if *domainFlag != domainLower {
			command.Check(fmt.Errorf("spell domain (%q) in lower-case, i.e. %q", *domainFlag, domainLower))
		}
	}

	rippled, err := cmd.Rippled()
	command.Check(err)

	// Learn needed details, i.e. account sequence number.
	remote, err := websockets.NewRemote(rippled)
	command.Check(err)
	defer remote.Close()

	command.Infof("Connected to %q\n", rippled) // debug

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult
	g.Go(func() error {
		var err error
		accountInfo, err = remote.AccountInfo(*asAccount)
		if err != nil {
			command.Errorf("Failed to get account_info %s: %s", asAccount, err)
			return err
		}
		return nil
	})
	err = g.Wait()
	command.Check(err)

	// Prepare to encode transaction output.
	txs := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txs)
	})

	// tx setters expect nil for unchanged
	if *domainFlag == unchanged {
		domainFlag = nil
	}
	if *memoFlag == "" {
		memoFlag = nil
	}

	t, err := tx.NewAccountSet(
		tx.SetAddress(asAccount),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12),
		tx.AddMemo(memoFlag), // TODO support multiple memo fields
		tx.AddMemo(memohex),
		tx.SetDomain(domainFlag),
		tx.SetCanonicalSig(true),
	)

	command.Check(err)

	// marshall the tx to stdout pipeline
	txs <- t
	close(txs)

	// Wait for all output to be encoded
	err = g.Wait()
	command.Check(err)

	return nil
}
