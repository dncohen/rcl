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
	"strings"

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

// AccountTag is a "fully qualified" account identifier, meaning an
// address and destination (or source) tag.
type AccountTag struct {
	Account data.Account
	Tag     uint32 // not *uint32, because we want to use AccountTag as map key
}

func NewAccountTag(acct data.Account, tag *uint32) AccountTag {
	this := AccountTag{Account: acct}
	if tag != nil {
		this.Tag = *tag
	}
	return this
}

func (this *AccountTag) String() string {
	if this == nil {
		log.Panic("nil AccountTag passed to AccountTag.String()")
	}
	if this.Tag == 0 { // treat 0 as no tag
		return this.Account.String()
	} else {
		return fmt.Sprintf("%s.%d", this.Account, this.Tag)
	}
}

var accountByNickname map[string]AccountTag

var nicknameByAccount map[AccountTag]string // would nicknameByAddress be more useful?

func initializeNicknames() error {
	// once
	if accountByNickname != nil {
		return nil
	}

	accountByNickname = make(map[string]AccountTag)
	nicknameByAccount = make(map[AccountTag]string)

	cfg, err := command.Config()
	if err != nil {
		return err
	}

	for _, section := range cfg.Sections() {
		if section.HasKey("address") {
			nickname := section.Name()
			address := section.Key("address").Value()

			var tag *uint32
			if section.HasKey("tag") {
				t, err := section.Key("tag").Uint()
				if err != nil {
					return fmt.Errorf("failed to parse RCL configuration %q: %w", section.Name(), err)
				}
				tmp := uint32(t)
				tag = &tmp
			}

			account, err := data.NewAccountFromAddress(address)
			if err != nil {
				return err
			}
			at := NewAccountTag(*account, tag)
			//log.Printf("account nickname %q: %v", nickname, at) // troubleshooting
			accountByNickname[nickname] = at
			nicknameByAccount[at] = nickname
			if at.Tag != 0 {
				// use nickname even when tag is not used
				noTag := NewAccountTag(*account, nil)
				_, ok := nicknameByAccount[noTag]
				if !ok {
					nicknameByAccount[noTag] = nickname
				}
			}
		}
	}

	return nil
}

// avoid scientific notation, not understood by ledger-cli
func formatValue(v data.Value) string {
	// unfortunately rubblelabs does not export data.Value:isScientific,
	// so we must manipulate strings
	str := v.String()
	if strings.Index(str, "e") != -1 {
		// simply using fmt.Sprintf("%f", v.Float()) produces, for example "0.000000"

		rat := v.Rat()
		// debug
		command.V(1).Infof("formatValue: converting scientific notation from %q to %q", v.String(), rat.FloatString(16))
		str = fmt.Sprintf("%s", rat.FloatString(16)) // TODO(dnc): proper decimal precision?
	}
	return str
}

// returns account nickname if known; otherwise, address string
func formatAccount(account data.Account, tag *uint32) string {
	at := NewAccountTag(account, tag)
	nick, ok := nicknameByAccount[at]
	if !ok && at.Tag != 0 {
		// fallback to nickname without tag
		at.Tag = 0
		nick, ok = nicknameByAccount[at]
	}
	if !ok {
		//log.Printf("no account nickname for %v", at) // troubleshooting
		// no nickname for this account
		return account.String()
	}
	return nick
}

// Helper for operations that expect a list of accounts.  We want to
// accept (and display) accounts by local nickname, as well as normal
// ripple address.
func parseAccountArg(arg []string) ([]AccountTag, error) {
	err := initializeNicknames()
	if err != nil {
		return nil, err
	}

	var account []AccountTag

	for _, a := range arg {
		acct, ok := accountByNickname[a]
		if !ok {
			tmp, err := data.NewAccountFromAddress(a)
			if err != nil {
				return account, fmt.Errorf("bad address (%q): %w", a, err)
			}
			acct = NewAccountTag(*tmp, nil)
		}
		account = append(account, acct)
	}

	return account, err
}
