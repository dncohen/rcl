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

// Command rcl-key
//
// The rcl-key command generates keys and signs transactions for the
// Ripple Consensus Ledger.
//
// Usage:
//
//    rcl-key [flags...] <operation> [operation flags...]
//
package main

import (
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/dncohen/rcl/internal/cmd"

	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

// Use `go get src.d10.dev/dumbdown` to fetch dumbdown tool.
//go:generate sh -c "go doc | dumbdown > README.md"

// Key is primarily used to marshal keys to/from files.
type Key struct {
	Account  data.Account `json:"address"`
	Secret   string       `json:"secret"`
	Nickname string       `json:"nickname"`
}

func SaveKeyToFile(key interface{}, filename string) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0400)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "\t")
	err = enc.Encode(key)
	return err
}

func ReadKeyFromFile(key interface{}, filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, key)
	return err
}

var (
	asFlag    *string
	asAccount *data.Account
)

func main() {
	command.RegisterCommand(command.Command{
		Application: "rcl",
		Description: "Generate keypairs and sign transactions.",
	})

	// for consistency with cmd/rcl-tx, define -as=<account> flag here (rather than sign operation)
	asFlag = command.CommandFlagSet.String("as", "", "Address of signing account")

	_, err := command.Config()
	if errors.Is(err, config.ConfigNotFound) {
		// not a problem, we'll use defaults
		// TODO(dnc): if -config specified explicitly on command line, make this a fatal error
		// command.Info(err)
		err = nil
	}
	// if error, fail and show usage
	command.CheckUsage(err)

	if *asFlag != "" {
		tmp, err := cmd.ParseAccountArg([]string{*asFlag})
		if err != nil {
			command.Check(fmt.Errorf("bad address (%q): %w", *asFlag, err))
		}
		asAccount = &tmp[0].Account
	}

	// this command requires an operation
	if len(flag.CommandLine.Args()) < 1 {
		command.CheckUsage(errors.New("command requires an operation"))
	}

	// default prefix for subcommand
	log.SetPrefix(fmt.Sprintf("rcl-key %s: ", flag.CommandLine.Args()[0]))

	err = command.CurrentOperation().Operate()
	command.CheckUsage(err)

	command.Exit()

}

// Probably belong in util.
func decodeInput() (*data.Transaction, error) {
	// GOBs are tricksy.
	// Decode into interface.
	var tx data.Transaction

	encoding := "gob64" // TODO make others optional, if needed.

	// Register instances of what we accept.
	// Should have all tx types here, I am putting them here when needed.
	gob.Register(&data.AccountSet{})
	gob.Register(&data.OfferCancel{})
	gob.Register(&data.OfferCreate{})
	gob.Register(&data.Payment{})
	gob.Register(&data.TrustSet{})

	var err error
	// NOTE currently only "gob64" is tested/working/used, and barely.
	if encoding == "gob64" {
		b64Reader := base64.NewDecoder(base64.StdEncoding, os.Stdin)
		err = gob.NewDecoder(b64Reader).Decode(&tx) // Decode into *pointer* to interface.
	} else if encoding == "gob" {
		err = gob.NewDecoder(os.Stdin).Decode(&tx)
	} else {
		err = fmt.Errorf("Unexpected encoding: %s\n", encoding)
	}

	if err != nil {
		return nil, err
	}

	return &tx, nil
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
