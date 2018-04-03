package main

// Saves a transaction with a reasonable file name.

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
)

func (s *State) save(args ...string) {
	const help = `

Save a transaction to disk.  Give it a reasonable file name.
`
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	// TODO allow specification of regular key
	// TODO -yes to skip prompts

	// Flags specific to this subcommand

	// this is the inbound encoding
	fs.String("encoding", "gob64", "Just use gob64.") // TODO replace with marshal helpers XXX

	s.saveCommand(fs)

}

func (s *State) saveCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " save: ")

	txPtr, err := decodeInput()
	if err != nil {
		s.Exit(err)
	}
	tx := *txPtr

	// Encode to file.
	// TODO put altnet in filename?
	filename := fmt.Sprintf("%s-%d-%s-%s.rcl.tx", tx.GetBase().Account, tx.GetBase().Sequence, tx.GetType(), tx.GetHash())
	f, err := os.Create(filename)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to create file %s", filename))
	}

	err = encodeOutput(&tx, fs, f)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to encode transaction to file"))
	}

	log.Printf("Transaction saved as %s.\n", filename)
}
