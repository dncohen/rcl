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

// Operation save
//
// Save a transaction to disk.  Give it a reasonable file name.
//
package main

// Saves a transaction with a reasonable file name.

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/util/marshal"
	"github.com/rubblelabs/ripple/data"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opSave,
		Name:        "save",
		Syntax:      "save ",
		Description: `Save a transaction to local filesystem.`,
	})
}

func opSave() error {

	// decode transactions from stdin
	txIn := make(chan (data.Transaction))
	go func() {
		err := marshal.DecodeTransactions(os.Stdin, txIn)
		if err != nil {
			if err == io.EOF {
				// Expected at end of input
				// TODO: ensure there's been at least one
			} else {
				command.Check(err)
			}
			close(txIn)
		}
	}()

	// prepare to encode transactions to stdout, so we can be part of a pipeline.
	var g errgroup.Group
	txOut := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txOut)
	})
	// Later, we will wait for g to complete.

	for tx := range txIn {
		// Encode to file.
		// TODO put altnet in filename?

		filename := fmt.Sprintf("rcl-tx-%s-%d-%s-%s.json", tx.GetBase().Account, tx.GetBase().Sequence, tx.GetType(), tx.GetHash())
		f, err := os.Create(filename)
		if err != nil {
			command.Check(fmt.Errorf("failed to create file %q: %w", filename, err))
		}

		err = encodeJSON(&tx, f)
		if err != nil {
			command.Check(fmt.Errorf("failed to save transaction to %q: %w\n", filename, err))
		} else {
			command.Infof("transaction saved as %s.\n", filename)

			// pipe the transaction only if able to save it
			txOut <- tx
		}
	}

	// Ensure we wait for all output to be encoded
	close(txOut)
	err := g.Wait()
	command.Check(err)

	return nil
}
