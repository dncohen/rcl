package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/tx"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

// Implements `trust` subcommand.  Create or modify a trust line.

func (s *State) trust(args ...string) {

	const help = `

Create or modify a trust line.

`

	fs := flag.NewFlagSet("trust", flag.ExitOnError)

	fs.Bool("ripple", false, "Allow rippling on account side of trustline.  Defaults to false which means disallow rippling.")
	fs.Bool("authorize", false, "Authorize trustline.  Defaults to false which means make no change to current authorization.")
	// TODO quality, rippling
	s.ParseFlags(fs, args, help, "trust <amount>")

	s.trustCommand(fs)
}

func (s *State) trustCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " trust: ")

	//log.Println(fs.Args()) // debug

	// command line flags
	allowRipple := boolFlag(fs, "ripple")
	_ = allowRipple // TODO XXX

	// command line args
	args := fs.Args()
	fail := false

	amount, err := data.NewAmount(args[0])
	if err != nil {
		log.Printf("Expected amount, got \"%s\" (%s)\n", args[1], err)
		fail = true
	} else if amount.IsNative() {
		log.Printf("Unxpected amount \"%s\".  Cannot set trust for XRP.\n", amount)
		fail = true
	}

	if asAccount == nil {
		fail = true
		fmt.Println("Use -as <address> flag to specify an account.")
		usageAndExit(flag.CommandLine)
	}

	if fail {
		s.ExitNow()
	}

	// TODO confirm
	log.Printf("Set trust %s ---> %s\n", asAccount, amount)

	rippled := config.GetRippled()
	if rippled == "" {
		log.Println("No rippled URL found in .cfg.")
		fail = true
	}

	// Learn the needed detail of the account setting the line.
	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to connect to %s", rippled))
	}
	defer remote.Close()

	log.Printf("Connected to %s\n", rippled) // debug

	// Interestingly, rubblelabs commands do not support server_info!
	// As it happens, the account_info returns ledger_current_index,
	// which allows us to compute a LastLedgerSequence.

	// Here we also call for fee... but don't actually use the result.

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult

	g.Go(func() error {
		var err error
		accountInfo, err = remote.AccountInfo(*asAccount)
		if err != nil {
			log.Printf("Failed to get account_info %s: %s", asAccount, err)
			return err
		}
		return nil
	})

	// not currently using...we could omit this.
	/*
		var feeInfo *websockets.FeeResult
			g.Go(func() error {
				var err error
				feeInfo, err = remote.Fee()
				if err != nil {
					log.Printf("Failed to get fee: %s", err)
					return err
				}
				return nil
			})
	*/
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}

	// Prepare to encode transaction output.
	txs := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txs)
	})

	// Prepare a TrustSet transaction.
	t, err := tx.NewTrustSet(
		tx.SetAddress(asAccount),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12),
		tx.SetLimitAmount(*amount),
		// TODO flags
		// TODO qualityin, qualityout
		tx.SetCanonicalSig(true),
	)

	// TODO: is it necessary to clean up the hash that rubblelabs puts into unsigned tx?
	// "hash":"0000000000000000000000000000000000000000000000000000000000000000"

	// Show in json format (debug)
	j, _ := json.MarshalIndent(t, "", "\t")
	log.Printf("Unsigned:\n%s\n", string(j))
	// In case user in on a terminal, nice to have a clean line.
	fmt.Fprintf(os.Stderr, "\n")

	// marshall the tx to stdout pipeline
	txs <- t
	close(txs)

	// Wait for all output to be encoded
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}
}
