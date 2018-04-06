package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/tx"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

var (
	zeroAccount data.Account
)

// send is a simple payment where the source and destination currency are the same.

func (s *State) send(args ...string) {
	const help = `

Send XRP or issuance.  This is a simple payment, meaning the source and destination currency is the same.
`

	fs := flag.NewFlagSet("send", flag.ExitOnError)
	// TODO flags
	s.ParseFlags(fs, args, help, "send <beneficiary> <amount>")
	s.sendCommand(fs)
}

func (s *State) sendCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " send: ")

	// command line args
	args := fs.Args()
	if len(args) != 2 {
		usageAndExit(fs)
	}
	errs := false

	arg := 0
	var tag *uint32
	beneficiary, tag, err := config.AccountFromArg(args[arg])
	if err != nil {
		log.Printf("Expected beneficiary address, got \"%s\" (%s)\n", args[arg], err)
		errs = true
	}
	arg++
	amount, err := config.AmountFromArg(args[arg])
	if err != nil {
		log.Printf("Expected amount, got \"%s\" (%s)\n", args[arg], err)
		errs = true
	}

	rippled := config.GetRippled()
	if rippled == "" {
		log.Println("No rippled URL found in rcl.cfg.")
		errs = true
	}

	if asAccount == nil {
		originatorAddress := config.GetAccount()
		if originatorAddress == "" {
			log.Println("No source account found in rcl.cfg.")
			errs = true
		}
		asAccount, err = data.NewAccountFromAddress(originatorAddress)
		if err != nil {
			log.Printf("Bad originator address \"%s\": %s\n", originatorAddress, err)
			errs = true
		}
	}

	if errs {
		s.ExitNow()
	}

	// Connect, to learn LastLedgerSequence and account sequence.
	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to connect to %s", rippled))
	}

	// TODO Want to close, but leads to "use of closed network connection" error.
	//defer remote.Close()

	// Use an errgroup in case we eventually need multiple calls, i.e. to get fee information.
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
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}

	// Ensure no ambiguity in amounts or issuers.
	var sendMax *data.Amount
	if !amount.IsNative() {
		if amount.Issuer == zeroAccount {
			glog.V(2).Infof("using %s as %s issuer", beneficiary, amount.Currency)
			amount.Issuer = *beneficiary
		}
		sendMax = amount
	}

	tx, err := tx.NewPayment(
		tx.SetAddress(asAccount),
		tx.SetSourceTag(asTag),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12), // TODO

		// Simple payment, source and destination currency the same.
		tx.SetAmount(amount),
		tx.SetSendMax(sendMax),

		tx.SetDestination(*beneficiary),
		tx.SetDestinationTag(tag),

		tx.SetCanonicalSig(true),
	)

	if glog.V(1) {
		// Show in json format (debug)
		j, _ := json.MarshalIndent(tx, "", "\t")
		glog.Infof("Unsigned:\n%s\n", string(j))
	}

	// Prepare to encode transaction output.
	txs := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txs)
	})

	// Pass unsigned transaction to encoder
	txs <- tx
	close(txs)

	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}

	glog.V(2).Infof("Prepared unsigned %s from %s to %s.\n", tx.GetType(), tx.Account, tx.Destination)
}
