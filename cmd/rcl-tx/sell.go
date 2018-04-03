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

// Implements `sell` subcommand of rcl-tx.  Creates an offer to sell
// one asset/issuance for another.

func (s *State) sell(args ...string) {

	const help = `

Create an offer to sell one asset or issuance for another.

`

	fs := flag.NewFlagSet("sell", flag.ExitOnError)

	s.ParseFlags(fs, args, help, "sell <amount> for <amount>")

	s.sellCommand(fs)
}

func (s *State) sellCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " sell: ")

	log.Println(fs.Args()) // debug

	// command line args
	args := fs.Args()
	if len(args) < 3 {
		s.Exitf(intro)
	}
	fail := false
	takerGets, err := data.NewAmount(args[0])
	if err != nil {
		log.Printf("Expected amount to sell, got \"%s\" (%s)\n", args[0], err)
		fail = true
	}

	takerPays, err := data.NewAmount(args[2])
	if err != nil {
		log.Printf("Expected 'taker pays' amount, got \"%s\" (%s)\n", args[1], err)
		fail = true
	}

	// Honor -as command flag
	if asAccount == nil {
		originatorAddress := config.GetAccount()
		if originatorAddress == "" {
			log.Println("No source account found in rcl.cfg.")
			fail = true
		}
		asAccount, err = data.NewAccountFromAddress(originatorAddress)
		if err != nil {
			log.Printf("Bad originator address \"%s\": %s\n", originatorAddress, err)
			fail = true
		}
	}

	if fail {
		s.ExitNow()
	}

	// Make the user type "for", less likely to mistakenly reverse the amounts.
	if args[1] != "for" {
		log.Println("Expected `sell <amount> for <amount>`.")
		s.ExitNow()
	}

	// TODO confirm
	log.Printf("Sell %s from %s in exchange for %s...\n", takerGets, asAccount, takerPays)

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

	// account_info returns ledger_current_index,
	// which allows us to compute a LastLedgerSequence.

	// Here we also call for fee... but don't yet use the result.

	var g errgroup.Group
	var accountInfo *websockets.AccountInfoResult
	var bookOffers *websockets.BookOffersResult
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

	g.Go(func() error {
		var err error
		bookOffers, err = remote.BookOffers(*asAccount, "validated", *takerPays.Asset(), *takerGets.Asset())
		if err != nil {
			log.Printf("Failed to get book_offers: %s", err)
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

	log.Printf("Order book has %d offers.\n", len(bookOffers.Offers)) // debug
	if len(bookOffers.Offers) > 0 {
		// This is work in progress... to detect whether our order is far off the current order book.  TODO
		top := bookOffers.Offers[0]
		log.Printf("top quality: %s", top.Quality)
		log.Printf("top taker pays %s, gets %s", top.TakerPays, top.TakerGets)
		topRatio := top.TakerPays.Ratio(*top.TakerGets)
		log.Printf("book ratio: %s\n", topRatio)

		ourRatio := takerPays.Ratio(*takerGets)
		log.Printf("Our offer ratio: %s", ourRatio)

		ratRatio, err := topRatio.Ratio(*ourRatio)
		if err != nil {
			s.Exit(err)
		}
		log.Printf("ratio of ratios: %s", ratRatio)

		// TODO: inspect other side of order book and determine if offer will cross.
		if ratRatio.Compare(one) > 0 {
			log.Println("Placing offer at TOP of order book.")
		}
		//q.Q(bookOffers.Offers[0])
		//s.Exitf("XXX")
	}

	// Prepare to encode transaction output.
	txs := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txs)
	})

	// Prepare transaction.
	offer, err := tx.NewOfferCreate(
		tx.SetAddress(asAccount),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12),

		tx.SetTakerPays(takerPays),
		tx.SetTakerGets(takerGets),
		tx.SetFlags(data.TxSell),

		tx.SetCanonicalSig(true),
	)

	// TODO: is it necessary to clean up the hash that rubblelabs puts into unsigned tx?
	// "hash":"0000000000000000000000000000000000000000000000000000000000000000"

	// Show in json format (debug)
	j, _ := json.MarshalIndent(offer, "", "\t")
	log.Printf("Unsigned:\n%s\n", string(j))

	// Pass unsigned transaction to encoder
	txs <- offer
	close(txs)

	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}
	//time.Sleep(10 * time.Second) // test
	//log.Println("exiting.")

	// In case user in on a terminal, nice to have a clean line.
	fmt.Fprintf(os.Stderr, "\n")
	log.Printf("Unsigned %s by %s prepared.\n", offer.GetType(), offer.GetBase().Account)
}
