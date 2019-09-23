// Copyright (C) 2019  David N. Cohen

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

// The rcl-data command retrieves historical data from data.ripple.com/v2/... and displays information about accounts on the Ripple Consensus Ledger.

// Each subcommand has a -help flag that explains it in more detail.  For instance

//   rcl-data show -help

// explains the purpose and usage of the show subcommand.

// There is a set of global flags such as -config to specify the
// configuration directory, where rcl-tx expects to find one or more
// *.cfg files.  These global flags apply to all subcommands.

// Each subcommand has its own set of flags, which if used must appear
// after the subcommand name.

// For a list of available subcommands and global flags, run

//   rcl-data -help
//
// Copyright(C) 2019  David N. Cohen see source code for license (AGPL 3)
package main

// generate dependency: `go get src.d10.dev/dumbdown`
//go:generate sh -c "go doc | dumbdown > README.md"

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

const (
	LedgerSequenceInterval = 10
	programName            = "rcl-data"
)

var (
	// Values useful for RCL math.
	zeroNative    *data.Value
	zeroNonNative *data.Value
	oneNative     *data.Value
	oneNonNative  *data.Value
)

var cfg config.Config

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	zeroNative, _ = data.NewNativeValue(0)
	zeroNonNative, _ = data.NewNonNativeValue(0, 0)
	oneNative, _ = data.NewNativeValue(1)
	oneNonNative, _ = data.NewNonNativeValue(1, 0)
}

// Parse the list of accounts on the command line.
func accountsFromArgs(args []string) (map[string]*data.Account, error) {
	if len(args) > 0 {
		accounts := make(map[string]*data.Account)

		// Each arg could be either address or nickname
		for _, arg := range args {
			section := cfg.Section(arg)
			address := section.Key("address").String()
			account, err := data.NewAccountFromAddress(address)
			if err != nil {
				return accounts, errors.Wrapf(err, "Bad account address: %s", arg)
			}
			accounts[arg] = account
		}
		return accounts, nil
	} else {
		// TODO: return all known accounts in config.
		return nil, fmt.Errorf("no account specified")
		//return config.GetAccountsByNickname(), nil
	}
}

func main() {
	command.RegisterCommand(command.Command{
		Application: `rcl-data`,
		Description: `Inspect historic RCL activity via Data API.`,
	})

	var err error
	cfg, err = command.Config()
	if errors.Cause(err) == config.ConfigNotFound {
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
	log.SetPrefix(fmt.Sprintf("rcl-data %s: ", flag.CommandLine.Args()[0]))

	err = command.CurrentOperation().Operate()
	command.CheckUsage(err)

	command.Exit()
}
