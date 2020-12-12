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

// Command rcl-key - Operation generate
//
// Generate new keypairs and addresses for use on the Ripple Consensus Ledger.
//
// Generated keys are saved to a file named '[ADDRESS].rcl-key', where
// [ADDRESS] is the address derived from the public key. The file is
// not encrypted, so handle with care.
//
// After generating `.rcl-key` files, run the `backup` operation.
// Doing so generates a `.cfg` file containing the public address and
// no secret.  It is this `.cfg` file that other rcl tools commands
// will read. It is recommended to keep `.rcl-key` files on an
// offline, secure machine for signing transactions.  The `.cfg` files
// can be copied to online machines where transactions are composed.
//
package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"golang.org/x/crypto/ssh/terminal"
	"src.d10.dev/command"
)

type key struct {
	seed     data.Seed
	keyType  data.KeyType
	seq      *uint32
	account  data.Account
	nickname string
}

func newKey(secret string) (*key, error) {
	seed, err := data.NewSeedFromAddress(secret) // assuming the "Address" is a typo in "NewSeedFromAddress
	if err != nil {
		return nil, err
	}

	// TODO(dnc): support all key types
	typ := data.ECDSA
	seq := uint32(0)
	acc := seed.AccountId(typ, &seq)

	return &key{
		seed:    *seed,
		keyType: typ,
		seq:     &seq,
		account: acc,
	}, nil
}

func generate(keyType data.KeyType, seq *uint32) (*key, error) {

	key := &key{
		keyType: keyType,
		seq:     seq,
	}

	seedBytes := make([]byte, len(key.seed))

	n, err := rand.Read(seedBytes)
	if err != nil {
		return key, err
	}
	if n != len(key.seed) {
		// Sanity check.
		return key, fmt.Errorf("Expected %d seed bytes, got %d\n", len(key.seed), n)
	}

	copy(key.seed[:], seedBytes)
	// Now seed has 16 bytes from crypt.Rand.

	secret, err := key.seed.Hash()
	if err != nil {
		return key, err
	}

	key.account = key.seed.AccountId(key.keyType, key.seq)

	// Sanity check
	sanity, err := data.NewSeedFromAddress(secret.String())
	if err != nil {
		return key, err
	}
	if *sanity != key.seed {
		return key, fmt.Errorf("Seed sanity check failure.")
	}

	return key, nil
}

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opGenerate,
		Name:        "generate",
		Syntax:      "generate [-n=<int>] [-vanity=<regex>] [-nickname=<nick>]",
		Description: `Operation "generate" creates a new RCL address with signing key.`,
	})
}

func opGenerate() error {

	nFlag := command.OperationFlagSet.Int("n", 1, "Number of keypairs to generate.")
	vanityFlag := command.OperationFlagSet.String("vanity", "", "optional regular expression to match")
	nicknameFlag := command.OperationFlagSet.String("nickname", "", "give generated address a nickname")
	secretFlag := command.OperationFlagSet.Bool("secret", false, "prompt for existing secret, instead of generating a new one")

	// TODO(dnc): choose any supported curve

	err := command.ParseOperationFlagSet()
	if err != nil {
		return err
	}

	if *nFlag <= 0 {
		return fmt.Errorf("count parameter (%d) must be positive number", *nFlag)
	}

	matched := make(chan *Key, 0) // addresses that match vanity expression
	unmatched := matched          // same channel if vanityFlag empty

	discards := 0
	pairs := 0
	timeouts := 0
	saves := 0

	if *vanityFlag != "" {
		exp, err := regexp.Compile(*vanityFlag)
		command.Check(err)
		command.V(1).Infof("Attempting to generate %d address matching %q.", *nFlag, *vanityFlag)

		// prepare to filter matches
		unmatched = make(chan *Key, *nFlag)
		go func() {
			for k := range unmatched {
				if exp.MatchString(k.Account.String()) {
					matched <- k
				} else {
					discards++
				}
			}
			close(matched)
		}()
	}

	var keyIn []*key
	for *secretFlag {
		if len(keyIn) == 0 {
			fmt.Printf("Enter RCL secret key: ")
		} else {
			fmt.Printf("Enter RCL secret key #%d (or return to continue): ", len(keyIn)+1)
		}
		b, err := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println("") // newline
		command.Check(err)

		if len(b) == 0 {
			if len(keyIn) == 0 {
				command.Check(errors.New("no secret provided, exiting"))
			}
			break
		}
		key, err := newKey(string(b))
		if err != nil {
			// err may leak secret key, so we do not show it here
			// fmt.Println(err)
			fmt.Println("bad secret key, please try again (or return to continue)")
			continue
		}

		keyIn = append(keyIn, key)
		fmt.Printf("secret for address %q entered\n", key.account)

		if len(keyIn) >= *nFlag {
			break
		}
	}

	workers := *nFlag
	if workers > runtime.NumCPU() {
		workers = runtime.NumCPU()
	}

	// generate key(s)
	for i := 0; i < workers; i++ {
		// start a worker
		go func() {
			for saves < *nFlag || saves < len(keyIn) {
				var key *key
				var err error
				if *secretFlag {
					key = keyIn[saves]
				} else {
					seq := uint32(0)
					key, err = generate(data.ECDSA, &seq)
				}
				command.Check(err)

				pairs++
				hash, err := key.seed.Hash()
				if err != nil {
					command.Error(err) // reached?
					continue
				}
				unmatched <- &Key{Account: key.account, Secret: hash.String()}
			}
			log.Println("worker exiting") // debug
		}()
	}

	timeout := 1 * time.Minute

	for saves < *nFlag {
		select {
		case k := <-matched:
			if saves >= *nFlag {
				command.Infof("extra key generated (not saved)") // debug
				continue
			}

			if *nicknameFlag != "" {
				if *nFlag == 1 {
					k.Nickname = *nicknameFlag
				} else {
					k.Nickname = fmt.Sprintf("%s-%d", *nicknameFlag, saves+1)
				}
			}

			// save the private key
			filename := fmt.Sprintf("%s.rcl-key", k.Account)
			err := SaveKeyToFile(k, filename)
			command.Check(err)
			command.Infof("Saved private key: %s", filename)
			saves++

		case <-time.After(timeout):
			// attempt to detect unmatchable regexp
			timeouts++
			if saves == 0 {
				command.Infof("No matches (%q) in %s.", *vanityFlag, time.Duration(timeouts)*timeout)
				if timeouts >= 5 {
					command.Check(fmt.Errorf("Giving up: no matches for %s", time.Duration(timeouts)*timeout))
				}
			} else {
				command.Infof("after %s, saved %d; unmatched %d of %d generated", timeout, saves, discards, pairs)
			}
		}
	}
	defer close(unmatched) // defer to avoid send on closed channel

	command.Infof("Saved %d keys, discarded %d of %d generated.", saves, discards, pairs)

	return nil
}
