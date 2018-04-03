package main

import (
	"flag"
	"log"
	"os"

	"github.com/dncohen/rcl/util"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/websockets"
	"github.com/y0ssar1an/q"
)

func (s *State) monitor(args ...string) {
	const help = `

Monitor RCL for transaction activity.

`

	// subcommand-specific flags
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	fs.Int("since", 0, "Ledger sequence number where monitoring will start. Defaults to most recent.")

	s.ParseFlags(fs, args, help, "monitor [-since=<int>]")

	s.monitorCommand(fs)
}

func (s *State) monitorCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " monitor: ")

	rippled := config.Section("").Key("rippled").String()
	if rippled == "" {
		s.Exitf("rippled websocket address not found in configuration file. Exiting.")
	}

	// A subscription lets us know when new ledgers are validated.
	subscription, err := util.NewSubscription(rippled)
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to connect to %s", rippled))
	}
	go subscription.Loop()
	log.Printf("Connected to %s\n", rippled) // debug

	min, max, err := subscription.Ledgers()
	if err != nil {
		s.Exit(errors.Wrapf(err, "Failed to get available ledgers from %s", rippled))
	}
	log.Printf("%s ledger history %d - %d\n", rippled, min, max)

	since := uint32(intFlag(fs, "since"))
	if since == 0 {
		since = max
	}
	if since < min || since > max {
		s.Exitf("Cannot start with ledger %d.  History available on %s is %d-%d.\n", since, subscription.Remote, min, max)
	}

	// Scan ledger indexes one by one, so as never to miss data.  We
	// want to inspect transactions in the order they occur (even if our
	// service is offline from time to time).  Use a channel to queue
	// which ledger instances need to be inspected.  Prepare it with the
	// first sequence to scan.
	ledgerIndexes := make(chan uint32, 1)
	ledgerIndexes <- since

	// Loop indefinitely
	for {

		select {

		case idx := <-ledgerIndexes:

			// Wait for the ledger sequence, if necessary.
			min, max, err := subscription.Ledgers()
			if err != nil {
				log.Printf("Failed query ledger history: %s\n", err)
				// try again.  TODO sleep first?
				go func(idx uint32) {
					ledgerIndexes <- idx
				}(idx)
				continue
			}

			if min > idx {
				log.Panicf("Failed to get ledger %d, available history is %d-%d.", idx, min, max)
			}
			if max < idx {
				//log.Printf("Waiting for ledger %d...\n", idx)
				seq := <-subscription.AfterSequence(idx)
				// log.Printf("...ledger %d now available.\n", seq)
				// Sanity check
				if seq != idx {
					log.Panicf("Unexpected %d returned from subscription.AfterSequence(%d).", seq, idx)
				}
			}

			//log.Printf("Requesting ledger %d transactions...", idx) // verbose
			ledgerResult, err := subscription.Remote.Ledger(idx, true)
			q.Q(ledgerResult)
			os.Exit(2)
			if err != nil {
				log.Println(errors.Wrapf(err, "Error requesting transactions in ledger %d.\n", idx))
				// Put the same ledger index back on the queue, so we try again later.
				go func(idx uint32) {
					ledgerIndexes <- idx
				}(idx)
				continue
			}

			// Scan for transactions we're interested in.
			for _, tx := range ledgerResult.Ledger.Transactions {
				nick, ok := config.GetAccountNickname(tx.GetBase().Account)
				if !ok {
					//log.Printf("Nickname not found for %s\n", tx.GetBase().Account)
					// Only show transactions affecting account in our config file.
					continue
				}

				// TODO are all tx validated?

				// Note these tx lack metadata! TODO, use account_tx or subscribe to transaction fee and inspect affected nodes.
				// With data returned by Ledger we can filter by Account signer, but not affected nodes.
				// TODO: Why is ledger always 0? <- because metadata all empty.

				log.Printf("%s - %s, %s (%s) sequence %d, %s in ledger %d",
					tx.GetHash(), tx.GetType(), tx.GetBase().Account, nick, tx.GetBase().Sequence, tx.MetaData.TransactionResult, idx /*tx.LedgerSequence*/)

				// TODO: was a marker returned, do we need to paginate?
			}
			// Add the next ledger sequence to our queue.
			go func(ledgerResult *websockets.LedgerResult) {
				ledgerIndexes <- ledgerResult.Ledger.LedgerHeader.LedgerSequence + 1
			}(ledgerResult)
		}
	}

}
