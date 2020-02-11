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

// Command RCL-account
//
// Inspect RCL accounts.
package main

// Use `go get src.d10.dev/dumbdown` to fetch dumbdown tool.
//go:generate sh -c "go doc | dumbdown > README.md"

import (
	"encoding/base64"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

const (
	LedgerSequenceInterval = 10
)

var (
	// Values useful for RCL math.
	zeroNative    *data.Value
	zeroNonNative *data.Value
	oneNative     *data.Value
	oneNonNative  *data.Value
)

var test, _ = data.NewNativeValue(0)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	zeroNative, _ = data.NewNativeValue(0)
	zeroNonNative, _ = data.NewNonNativeValue(0, 0)
	oneNative, _ = data.NewNativeValue(1)
	oneNonNative, _ = data.NewNonNativeValue(1, 0)
}

func main() {
	command.RegisterCommand(command.Command{
		Application: "rcl",
		Description: "Inspect account(s) on Ripple Consensus Ledger.",
	})

	_, err := command.Config()
	if errors.Is(err, config.ConfigNotFound) {
		// not a problem, we'll use defaults
		// TODO(dnc): if -config specified explicitly on command line, make this a fatal error
		command.Info(err)
		err = nil
	}
	command.CheckUsage(err)

	// this command requires an operation
	if len(flag.CommandLine.Args()) < 1 {
		command.CheckUsage(errors.New("command requires an operation"))
	}

	// default prefix for subcommand
	log.SetPrefix(fmt.Sprintf("rcl-account %s: ", flag.CommandLine.Args()[0]))

	err = command.CurrentOperation().Operate()
	command.CheckUsage(err)

	command.Exit()

}

func encodeOutput(tx *data.Transaction, fs *flag.FlagSet, f *os.File) error {
	if f == nil {
		f = os.Stdout
	}
	encoding := "gob64" // currently only method supported

	var err error
	if encoding == "gob64" {
		// GOB is preferable as it preserves the type of the tx we've created.
		// However it is not terminal safe.  So we further encode to base64.
		b64Writer := base64.NewEncoder(base64.StdEncoding, f)
		defer b64Writer.Close()                    // Close() is important!!
		err = gob.NewEncoder(b64Writer).Encode(tx) // Encode a *pointer* to the interface.
	} else {
		err = fmt.Errorf("Unexpected encoding: %s", encoding)
	}
	return err
}
