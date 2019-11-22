// Copyright (C) 2019  David N. Cohen
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

// The rcl-data command retrieves historical data from
// data.ripple.com/v2/... and displays information about accounts on
// the Ripple Consensus Ledger.
//
// Each subcommand has a -help flag that explains it in more detail.  For instance
//
//   rcl-data show -help
//
// explains the purpose and usage of the show subcommand.
//
// There is a set of global flags such as -config to specify the
// configuration directory, where rcl-data expects to find one or more
// *.cfg files.  These global flags apply to all subcommands.
//
// Each subcommand has its own set of flags, which if used must appear
// after the subcommand name.
//
// For a list of available subcommands and global flags, run
//
//   rcl-data -help
//
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

// Use `go get src.d10.dev/dumbdown` to fetch dumbdown tool.
//go:generate sh -c "go doc | dumbdown > README.md"

func main() {
	command.RegisterCommand(command.Command{
		Application: "rcl-data",
		Description: "Inspect historical Ripple Consensus Ledger activity.",
	})

	_, err := command.Config()
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

var accountByNickname map[string]data.Account
var nicknameByAccount map[data.Account]string // would nicknameByAddress be more useful?

func initializeNicknames() error {
	// once
	if accountByNickname != nil {
		return nil
	}

	accountByNickname = make(map[string]data.Account)
	nicknameByAccount = make(map[data.Account]string)

	cfg, err := command.Config()
	if err != nil {
		return err
	}

	for _, section := range cfg.Sections() {
		if section.HasKey("address") {
			nickname := section.Name()
			address := section.Key("address").Value()
			account, err := data.NewAccountFromAddress(address)
			if err != nil {
				return err
			}
			accountByNickname[nickname] = *account
			nicknameByAccount[*account] = nickname
		}
	}

	return nil
}

// returns account nickname if known; otherwise, address string
func formatAccount(account data.Account) string {
	nick, ok := nicknameByAccount[account]
	if !ok {
		return account.String()
	}
	return nick
}

// Helper for operations that expect a list of accounts.  We want to
// accept (and display) accounts by local nickname, as well as normal
// ripple address.
func parseAccountArg(arg []string) ([]data.Account, error) {
	err := initializeNicknames()
	if err != nil {
		return nil, err
	}

	var account []data.Account
	for _, a := range arg {
		acct, ok := accountByNickname[a]
		if !ok {
			tmp, err := data.NewAccountFromAddress(a)
			if err != nil {
				return account, fmt.Errorf("bad address (%q): %w", a, err)
			}
			acct = *tmp
		}
		account = append(account, acct)
	}
	return account, err
}
