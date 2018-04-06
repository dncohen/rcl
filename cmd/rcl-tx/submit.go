package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/util"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func (s *State) submit(args ...string) {
	const help = `

Submit command broadcasts signed transactions to a rippled server.
`
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	// Flags specific to this subcommand (currently none)

	fs.Parse(args)

	s.submitCommand(fs)
}

func (s *State) submitCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " submit: ")

	// Read incoming signed transactions from stdin
	signedTransactions := make(chan (data.Transaction))
	go func() {
		err := marshal.DecodeTransactions(os.Stdin, signedTransactions)
		if err != nil {
			if err == io.EOF {
				// Expected at end of input
				// TODO: ensure there's been at least one
			} else {
				log.Println(err)
				s.Exit(err)
			}
			close(signedTransactions)
		}
	}()

	rippled := config.GetRippled()
	if rippled == "" {
		s.Exitf("No rippled URL found in rcl.cfg.")
	}

	// Subscription makes it easier to wait for transaction validation.
	//log.Printf("Connecting to %s...\n", rippled) // debug
	subscription, err := util.NewSubscription(rippled)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to connect to %s", rippled))
	}
	go subscription.Loop()
	log.Printf("Connected to %s\n", rippled) // debug

	// errgroup in order to wait for results of all submitted tx.
	var g errgroup.Group

	// Submit all transaction decoded
	for tx := range signedTransactions {
		tx := tx // scope (needed ?)
		// TODO? show user tx details and prompt to continue.

		if glog.V(3) {
			// Show the signed tx in JSON format. (verbose debug)
			jb, err := json.MarshalIndent(tx, "", "\t")
			if err != nil {
				glog.Errorln("Failed to encode signed transaction (json): ", err)
			} else {
				glog.Infof("Transaction JSON: \n%s\n", string(jb))
			}
		}

		g.Go(func() error {
			tx := tx // Scope (needed?)
			result, err := subscription.SubmitWait(tx)
			if err != nil {
				err = errors.Wrapf(err, "Failed to submit %s transaction %s", tx.GetType(), tx.GetHash())
				log.Println(err)
				return err
			}

			if !result.Validated {
				return fmt.Errorf("%s transaction %s tentative %s failed to validate!", tx.GetType(), tx.GetHash(), result.MetaData.TransactionResult)
			} else {
				// Show result of validated transaction.
				msg := fmt.Sprintf("%s %s (%s/%d) %s in ledger %d.\n", tx.GetType(), tx.GetHash(), tx.GetBase().Account, tx.GetBase().Sequence, result.MetaData.TransactionResult, result.LedgerSequence)
				if result.MetaData.TransactionResult.Success() {
					glog.Infof(msg)
				} else {
					glog.Errorf(msg)
				}
				fmt.Println(msg) // stdout
			}

			return err
		})
	}

	// Wait for result of each submitted transaction.
	err = g.Wait()
	if err != nil {
		log.Println(err)
		//s.Exit(err)
	}
}
