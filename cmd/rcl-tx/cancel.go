package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/tx"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

// Implements `cancel` subcommand of rcl-tx.
// Cancels an earlier offer.

func (s *State) cancel(args ...string) {

	const help = `

Cancel an offer to sell one asset or issuance for another.

`

	fs := flag.NewFlagSet("sell", flag.ExitOnError)
	fs.Bool("all", false, "Cancel all outstanding offers for an account.")
	s.ParseFlags(fs, args, help, "cancel <sequence> [<sequence> ...]")

	s.cancelCommand(fs)
}

func (s *State) cancelCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " cancel: ")

	// command line args
	all := boolFlag(fs, "all")
	args := fs.Args()
	if !all && len(args) < 1 {
		s.Exitf(intro)
	}
	fail := false

	seqs := make([]uint32, 0)
	for _, arg := range args {
		i, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Printf("Expected account sequence number, got %s: %s", arg, err)
			fail = true
		} else if i < 1 {
			fmt.Printf("Expected account sequence number, got %d (less than 1)", i)
			fail = true
		}
		seqs = append(seqs, uint32(i))
	}

	// Honor -as command flag
	/* now in main.go...
	if asAccount == nil {
		originatorAddress := config.GetAccount()
		if originatorAddress == "" {
			log.Println("No source account found in rcl.cfg.")
			fail = true
		}
		var err error
		asAccount, err = data.NewAccountFromAddress(originatorAddress)
		if err != nil {
			log.Printf("Bad originator address \"%s\": %s\n", originatorAddress, err)
			fail = true
		}
	}
	*/
	if asAccount == nil {
		fail = true
		fmt.Println("Use -as <address> flag to specify an account.")
		usageAndExit(flag.CommandLine)
	}

	if fail {
		s.ExitNow()
	}

	rippled := config.GetRippled()
	if rippled == "" {
		log.Println("No rippled URL found in rcl.cfg.")
		s.ExitNow()
	}

	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to connect to %s", rippled))
	}
	defer remote.Close()

	log.Printf("Connected to %s\n", rippled) // debug

	if all {
		// Cancel all of an account's transactions.
		result, err := remote.AccountOffers(*asAccount, "current")
		if err != nil {
			s.Exit(errors.Wrapf(err, "account_offers failed for %s: %s", asAccount))
		}
		for _, offer := range result.Offers {
			seqs = append(seqs, offer.Sequence)
		}
	}

	if len(seqs) < 1 {
		log.Println("No offers - nothing to do.")
		s.ExitNow()
	}

	// TODO confirm
	log.Printf("Cancel %d offer(s) by %s...\n", len(seqs), asAccount)

	// account_info returns ledger_current_index,
	// which allows us to compute a LastLedgerSequence.

	// Here we also call for fee... but don't yet use the result.

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult
	//var feeInfo *websockets.FeeResult

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

	// Prepare transactions.
	sequence := *accountInfo.AccountData.Sequence
	for _, offerSeq := range seqs {
		t, err := tx.NewOfferCancel(
			tx.SetAddress(asAccount),
			tx.SetSequence(sequence),
			tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
			tx.SetFee(12),
			tx.SetOfferSequence(offerSeq),
			tx.SetCanonicalSig(true),
		)
		if err != nil {
			log.Printf("Failed to prepare OfferCancel: %s", err)
			s.Exit(err)
		}
		sequence++
		// TODO: is it necessary to clean up the hash that rubblelabs puts into unsigned tx?
		// "hash":"0000000000000000000000000000000000000000000000000000000000000000"

		// Show in json format (debug)
		j, _ := json.MarshalIndent(t, "", "\t")
		log.Printf("Unsigned:\n%s\n", string(j))
		// In case user in on a terminal, nice to have a clean line.
		fmt.Fprintf(os.Stderr, "\n")

		log.Printf("Unsigned %s by %s prepared.\n", t.GetType(), t.GetBase().Account)

		// Pass unsigned transaction to encoder
		txs <- t
	}
	close(txs)

	// Wait for all output to be encoded
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}

}
