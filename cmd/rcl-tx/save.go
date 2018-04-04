package main

// Saves a transaction with a reasonable file name.

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/util/marshal"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func (s *State) save(args ...string) {
	const help = `

Save a transaction to disk.  Give it a reasonable file name.
`
	fs := flag.NewFlagSet("save", flag.ExitOnError)

	// Flags specific to this subcommand

	s.saveCommand(fs)

}

func (s *State) saveCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " save: ")

	// decode transactions from stdin
	txIn := make(chan (data.Transaction))
	go func() {
		err := marshal.DecodeTransactions(os.Stdin, txIn)
		if err != nil {
			if err == io.EOF {
				// Expected at end of input
				// TODO: ensure there's been at least one
			} else {
				log.Println(err)
				s.Exit(err)
			}
			close(txIn)
		}
	}()

	// prepare to encode transactions to stdout, so we can be part of a pipeline.
	var g errgroup.Group
	txOut := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txOut)
	})
	// Later, we will wait for g to complete.
	fail := false

	for tx := range txIn {
		// Encode to file.
		// TODO put altnet in filename?
		filename := fmt.Sprintf("rcl-tx-%s-%d-%s-%s.json", tx.GetBase().Account, tx.GetBase().Sequence, tx.GetType(), tx.GetHash())
		f, err := os.Create(filename)
		if err != nil {
			s.Exit(errors.Wrapf(err, "Failed to create file %s", filename))
		}

		err = encodeJSON(&tx, f)
		if err != nil {
			log.Printf("Failed to save transaction to %s: %s\n", filename, err)
			fail = true
		} else {
			log.Printf("Transaction saved as %s.\n", filename)
			// pipe the transaction only if able to save it
			txOut <- tx
		}
	}

	// Ensure we wait for all output to be encoded
	close(txOut)
	err := g.Wait()
	if err != nil {
		s.Exit(err)
	}
	if fail {
		s.ExitCode = 1
		s.ExitNow()
	}
}
