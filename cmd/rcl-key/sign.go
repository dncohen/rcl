package main

import (
	"encoding/json"
	"flag"
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

func (s *State) sign(args ...string) {
	const help = `

Sign command expects an encoded unsigned transaction via stdin, and encodes a signed transaction to stdout.
`

	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	// Flags specific to this subcommand
	fs.String("as", "", "Sign as address.  Allows for regular key signing.")
	err := fs.Parse(args)
	if err != nil {
		s.Exit(err)
	}

	s.signCommand(fs)

}

func getSigningKey(tx data.Transaction) (util.Keypair, error) {
	// Config helper to read secrets from *.cfg files.
	return config.GetAccountKeypair(tx.GetBase().Account)
}

func (s *State) signCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " sign: ")

	// allow specification of regular key
	var keypair util.Keypair
	asAddress := stringFlag(fs, "as")

	if asAddress != "" {
		account, _, err := config.AccountFromArg(asAddress)
		if err != nil {
			s.Exit(errors.Wrapf(err, "bad account %s", asAddress))
		}
		keypair, err = config.GetAccountKeypair(*account)
		if err != nil {
			s.Exit(errors.Wrapf(err, "cannot sign as %s", asAddress))
		}
	}

	// decode unsigned transactions from stdin
	unsignedTransactions := make(chan (data.Transaction))
	go func() {
		err := marshal.DecodeTransactions(os.Stdin, unsignedTransactions)
		if err != nil {
			if err == io.EOF {
				// Expected at end of input
				// TODO: ensure there's been at least one
			} else {
				log.Println(err)
				s.Exit(err)
			}
			close(unsignedTransactions)
		}
	}()

	// encode signed transactions to stdout
	var g errgroup.Group
	signedTransactions := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, signedTransactions)
	})

	// accept unsinged transactions from stdin
	var err error
	for tx := range unsignedTransactions {
		if asAddress == "" {
			keypair, err = getSigningKey(tx)
			if err != nil {
				s.Exit(errors.Wrapf(err, "Failed to determine signing key"))
			}
		}

		if tx.GetBase().Account.String() != keypair.Address {
			// Could be regular key or multisign, so this is not always an error.
			glog.Infof("Transaction %s account %s differs from signing key %s.\n", tx.GetType(), tx.GetBase().Account, keypair.Address)
		}

		// TODO show user tx details and prompt to continue.

		// Sign the transaction.
		err = keypair.Sign(tx)
		if err != nil {
			s.Exit(errors.Wrapf(err, "failed to sign transaction"))
		}
		if glog.V(2) {
			glog.Infof("%s %s signed by %s.\n", tx.GetType(), tx.GetHash(), keypair.Address)

			// Show the signed tx in JSON format (verbose debug)
			jb, err := json.MarshalIndent(tx, "", "\t")
			if err != nil {
				glog.Errorln("Failed to JSON-encode signed transaction: ", err)
			} else {
				glog.Infof("Signed transaction JSON: \n%s\n", string(jb))
			}
		}

		// Write to output
		signedTransactions <- tx
	}

	// This close cannot be defered because encode goroutine will not complete.
	close(signedTransactions)

	// Wait for all output to be encoded.
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}

}
