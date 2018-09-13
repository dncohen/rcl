package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/tx"
	"github.com/dncohen/rcl/util/marshal"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

// Implements `set` subcommand.  Set fields and flags for an account.
// At the moment, few of the fields described on
// https://developers.ripple.com/accountset.html are supported.

const (
	unchanged = "UNCHANGED"
)

func (s *State) set(args ...string) {

	const help = `

Set flag or a field on an account.

`

	fs := flag.NewFlagSet("set", flag.ExitOnError)

	fs.String("domain", unchanged, "The domain that owns this account, in lower case.")

	// Memo fields allowed on any transaction. So this logic should be
	// moved to main.go or somewhere else common to each rcl-tx
	// subcommand.  Also TODO, support multiple memos per transaction.
	fs.String("memo", "", "A memo string, to be hex encoded and written to ledger with the transaction.")
	fs.String("memohex", "", "A memo string, already hex encoded, to be written to ledger with the transaction.")

	s.ParseFlags(fs, args, help, "set [-domain <example.com>]")

	s.setCommand(fs)
}

func (s *State) setCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " set: ")
	fail := false

	domainFlag := stringFlag(fs, "domain")
	var domain *string

	if domainFlag != unchanged {
		domainLower := strings.ToLower(domainFlag)
		if domainFlag != domainLower {
			log.Printf("Expected domain in lower case, got \"%s\".  Try \"%s\".", domainFlag, domainLower)
			fail = true
		} else {
			domain = &domainFlag
		}
	}

	memoFlag := stringFlag(fs, "memo")
	var memo *string
	if memoFlag != "" {
		memo = &memoFlag
	}

	memohexFlag := stringFlag(fs, "memohex")
	memohexBytes := []byte(memohexFlag)
	var memohex []byte
	if memohexFlag != "" {
		memohex = make([]byte, hex.DecodedLen(len(memohexBytes)))
		_, err := hex.Decode(memohex, memohexBytes)
		if err != nil {
			log.Printf("Failed to decode hex memo (\"%s\")\n", memohexFlag)
			fail = true
		}
	}
	log.Println(memohex) // debug

	if asAccount == nil {
		fail = true
		fmt.Println("Use -as <address> flag to specify an account.")
		usageAndExit(flag.CommandLine)
	}

	rippled := config.GetRippled()
	if rippled == "" {
		log.Println("No rippled URL found in .cfg.")
		fail = true
	}

	if fail {
		s.ExitNow()
	}

	// Learn needed details, i.e. account sequence number.
	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to connect to %s", rippled))
	}
	defer remote.Close()

	log.Printf("Connected to %s\n", rippled) // debug

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

	// Prepare to encode transaction output.
	txs := make(chan (data.Transaction))
	g.Go(func() error {
		return marshal.EncodeTransactions(os.Stdout, txs)
	})

	t, err := tx.NewAccountSet(
		tx.SetAddress(asAccount),
		tx.SetSequence(*accountInfo.AccountData.Sequence),
		tx.SetLastLedgerSequence(accountInfo.LedgerSequence+LedgerSequenceInterval),
		tx.SetFee(12),
		tx.AddMemo(memo), // TODO support multiple memo fields
		tx.AddMemo(memohex),
		//tx.AddMemo(asAccount),
		tx.SetDomain(domain),
		tx.SetCanonicalSig(true),
	)
	_ = memo

	if err != nil {
		s.Exit(err)
	}

	if glog.V(2) {
		// Show in json format (debug)
		j, _ := json.MarshalIndent(t, "", "\t")
		log.Printf("Unsigned:\n%s\n", string(j))
		// In case user in on a terminal, nice to have a clean line.
		fmt.Fprintf(os.Stderr, "\n")
	}

	// marshall the tx to stdout pipeline
	txs <- t
	close(txs)

	// Wait for all output to be encoded
	err = g.Wait()
	if err != nil {
		s.Exit(err)
	}

}
