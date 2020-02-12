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

package cmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/go-ini/ini"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

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
var accountConfig map[AccountTag]*ini.Section

func initializeNicknames() error {
	// once
	if accountByNickname != nil {
		return nil
	}

	accountByNickname = make(map[string]AccountTag)
	accountConfig = make(map[AccountTag]*ini.Section)

	cfg, err := command.Config()
	if err != nil {
		if errors.Is(err, config.ConfigNotFound) {
			// no config, no nicknames
			return nil
		}
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
			accountConfig[at] = section

			if at.Tag != 0 {
				// use nickname even when tag is not used
				noTag := NewAccountTag(*account, nil)
				_, ok := accountConfig[noTag]
				if !ok {
					accountConfig[noTag] = section
				}
			}
		}
	}

	return nil
}

// returns account nickname if known; otherwise, address string
func FormatAccount(account data.Account, tag *uint32) string {
	nick, _ := accountDetail(account, tag)
	return nick
}

func accountDetail(account data.Account, tag *uint32) (nick, ledger string) {
	at := NewAccountTag(account, tag)
	cfg, ok := accountConfig[at]
	if !ok && tag != nil {
		// fallback to nickname without tag
		at.Tag = 0
		cfg, ok = accountConfig[at]
	}
	if !ok {
		//log.Printf("no account nickname for %v in %v", at, accountConfig) // troubleshooting
		// no nickname for this account
		return account.String(), ""
	}
	nick = cfg.Name()
	if cfg.HasKey("ledger") {
		ledger = cfg.Key("ledger").Value()
	}
	return
}

// Helper for operations that expect a list of accounts.  We want to
// accept (and display) accounts by local nickname, as well as normal
// ripple address.
func ParseAccountArg(arg []string) ([]AccountTag, error) {
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
