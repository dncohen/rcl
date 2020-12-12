// Copyright (C) 2019-2020  David N. Cohen
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

// Command rcl-key - Operation sign
//
// Sign command expects an encoded unsigned transaction via stdin, and
// encodes a signed transaction to stdout.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/pipeline"
	"github.com/dncohen/rcl/util"
	"github.com/rubblelabs/ripple/data"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opSign,
		Name:        "sign",
		Syntax:      "sign [<filename> ...]",
		Description: `Sign RCL transactions.  Unsigned transactions are read from stdin or files.  Signed transactions are written to stdout.`,
	})
}

func opSign() error {
	err := command.ParseOperationFlagSet()
	if err != nil {
		return err
	}

	argument := command.OperationFlagSet.Args()

	unsignedIn := make(chan data.Transaction)
	signedOut := make(chan data.Transaction)

	go func() {
		// sign all transactions in the pipeline
		for unsigned := range unsignedIn {
			signed, err := sign(unsigned)
			command.Check(err)
			signedOut <- signed
		}
		close(signedOut)
	}()

	go func() {
		// push incoming transaction onto pipeline
		if len(argument) == 0 {
			err := pipeline.DecodeInput(unsignedIn, os.Stdin)
			command.Check(err)
		} else {
			// read files
			for _, arg := range argument {
				match, err := filepath.Glob(arg)
				command.Check(err)
				if len(match) == 0 {
					command.Check(fmt.Errorf("expected transaction: file not found (%q)", arg))
				}
				for _, fname := range match {

					func() { // for defer
						f, err := os.Open(fname)
						command.Check(err)
						defer f.Close()

						err = pipeline.DecodeInput(unsignedIn, f)
						command.Check(err)
					}()
				}
			}
			// end of files
		}
		// files or stdin has been read
		close(unsignedIn)
	}()

	// the goroutines (above) produce signed transactions.  here we write them to stdout
	err = pipeline.EncodeOutput(os.Stdout, signedOut)
	command.Check(err)

	return nil
}

var keycache = make(map[data.Account]*Key)

func sign(unsigned data.Transaction) (data.Transaction, error) {
	var signer data.Account
	if *asFlag != "" {
		signer = *asAccount
	} else {
		// learn signer from transaction
		signer = unsigned.GetBase().Account
	}

	k, ok := keycache[signer]
	if !ok {
		// TODO(dnc): check current directory and also config directory.
		filename := fmt.Sprintf("%s.rcl-key", signer)
		k = &Key{}
		err := ReadKeyFromFile(k, filename)
		command.Check(err)
		keycache[signer] = k // cache for signing multiple tx
	}

	kp, err := util.NewEcdsaFromSecret(k.Secret)
	if err != nil {
		return nil, err
	}

	err = kp.Sign(unsigned)
	return unsigned, err
}
