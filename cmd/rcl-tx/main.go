// Copyright (C) 2019, 2020  David N. Cohen

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Command rcl-tx
//
// The rcl-tx command composes transactions for the Ripple Consensus
// Ledger.
//
// Each subcommand has a -help flag that explains it in more detail.  For
// instance
//
//     rcl-tx sell -help
//
// explains the purpose and usage of the sell subcommand.
//
// There is a set of global flags such as -config to specify the
// configuration directory, where rcl-tx expects to find one or more
// *.cfg files.  These global flags apply to all subcommands.
//
// Each subcommand has its own set of flags, which if used must appear
// after the subcommand name.
//
// For a list of available subcommands and global flags, run
//
//     rcl-tx -help
//
package main

// use `go get src.d10.dev/dumbdown`
//go:generate sh -c "go doc | dumbdown > README.md"

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

var (
	// transacting account, when composing transactions
	asFlag    *string
	asAccount *data.Account
	asTag     *uint32

	// note field common to all transaction creating operations
	memoFlag    *string
	memohexFlag *string
	memohex     []byte
)

const (
	LedgerSequenceInterval = 10
)

var (
	// Helpers for value math
	one data.Value
)

func init() {
	tmp, err := data.NewNonNativeValue(1, 0)
	if err != nil {
		log.Panic(err)
	}
	one = *tmp
}

func main() {
	command.RegisterCommand(command.Command{
		Application: `rcl`,
		Description: `Construct and submit Ripple Consensus Ledger (XRP Ledger) transactions.`,
	})

	// transaction operations accept -as <account>
	// TODO(dnc): default from config
	asFlag = command.CommandFlagSet.String("as", "", "address of transacting account (i.e. sender of payment)")

	// TODO(dnc): support multiple memo per tx

	memoFlag = command.CommandFlagSet.String("memo", "", "note, to be hex encoded and written to ledger with a transaction")
	memohexFlag = command.CommandFlagSet.String("memohex", "", "note, already hex encoded")

	// note, command.Config() calls command.CommandFlagSet.Parse()
	_, err := command.Config()
	if errors.Cause(err) == config.ConfigNotFound {
		// not a problem, we'll use defaults
		// TODO(dnc): if -config specified explicitly on command line, make this a fatal error
		command.Info(err)
		err = nil
	}
	// if error, fail and show usage
	command.CheckUsage(err)

	if *asFlag != "" {
		tmp, err := cmd.ParseAccountArg([]string{*asFlag})
		if err != nil {
			command.Check(fmt.Errorf("bad address (%q): %w", *asFlag, err))
		}

		asAccount, asTag = &tmp[0].Account, &tmp[0].Tag
	}

	if *memohexFlag != "" {
		memohex = make([]byte, hex.DecodedLen(len([]byte(*memohexFlag))))
		_, err := hex.Decode(memohex, []byte(*memohexFlag))
		if err != nil {
			command.Check(fmt.Errorf("bad memohex (%q): %w", *memohexFlag, err))
		}
	}

	// this command requires an operation
	if len(flag.CommandLine.Args()) < 1 {
		command.CheckUsage(errors.New("command requires an operation"))
	}

	// default prefix for subcommand
	log.SetPrefix(fmt.Sprintf("rcl-tx %s: ", flag.CommandLine.Args()[0]))

	err = command.CurrentOperation().Operate()
	command.CheckUsage(err)

	command.Exit()

}

// Encode a transaction to JSON.  A helper function for debug output
// and saving to file.  Note that when in pipeline, transactions
// should be encoded and decode by the util/marshal helper package.
func encodeJSON(tx *data.Transaction, f *os.File) error {
	var err error
	if f == nil {
		f = os.Stdout
	}

	j, _ := json.MarshalIndent(tx, "", "\t")
	_, err = fmt.Fprintln(f, string(j))
	return err
}
