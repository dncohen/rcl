package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

	"golang.org/x/sync/errgroup"

	"github.com/dncohen/rcl/util"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func (s *State) monitor(args ...string) {
	const help = `

Monitor RCL for transaction activity related to an account.

`

	// subcommand-specific flags
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	fs.Int("since", 0, "Ledger sequence number where monitoring will start. Defaults to most recent.")

	s.ParseFlags(fs, args, help, "monitor [-since=<int>]")

	s.monitorCommand(fs)
}

var (
	// Which accounts to monitor?
	accounts map[string]*data.Account
)

func accountsToMonitor(args []string) (map[string]*data.Account, error) {
	if len(args) > 0 {
		accounts = make(map[string]*data.Account)

		// Each arg could be either address or nickname
		for _, arg := range args {
			account, _, ok := config.GetAccountByNickname(arg)
			if !ok {
				var err error
				account, err = data.NewAccountFromAddress(arg)
				if err != nil {
					return accounts, errors.Wrapf(err, "Bad account address: %s", arg)
				}
			}
			accounts[arg] = account
		}
		return accounts, nil
	} else {
		// TODO get all accounts from config
		return nil, fmt.Errorf("no account specified")
		//return config.GetAccountsByNickname(), nil
	}
}

func (s *State) monitorCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " monitor: ")

	rippled := config.Section("").Key("rippled").String()
	if rippled == "" {
		s.Exitf("rippled websocket address not found in configuration file. Exiting.")
	}

	accounts, err := accountsToMonitor(fs.Args())
	if err != nil {
		s.Exit(err)
	}
	if len(accounts) == 0 {
		log.Println("No accounts specified")
		s.ExitNow()
	}
	log.Printf("Monitoring %d accounts", len(accounts))

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

	var since uint32
	switch i := intFlag(fs, "since"); i {
	case -1:
		since = min
	case 0:
		since = max
	default:
		since = uint32(i)
	}

	if since < min || since > max {
		s.Exitf("Cannot start with ledger %d.  History available on %s is %d-%d.\n", since, rippled, min, max)
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

			//log.Printf("calling account_tx for ledger %d\n", idx) // debug verbose

			// Map used to order transactions within the specific ledger.
			txs := make(map[uint32]*data.TransactionWithMetaData)
			_ = txs
			g := new(errgroup.Group)
			for _, acct := range accounts {
				g.Go(func() error {
					//log.Printf("requesting %d", idx) // debug
					txChan := subscription.Remote.AccountTx(*acct, 10, int64(idx), int64(idx))
					for tx := range txChan {
						// transactions will be shown in order they are applied to ledger.
						txs[tx.MetaData.TransactionIndex] = tx
					}
					return nil
				})
			}
			// Wait for all account_tx calls to return
			err = g.Wait()
			if err != nil {
				// Not sure whether to retry here?
				log.Printf("Failed to get tx ledger %d: %s\n", idx, err)
				// Put the same ledger index back on the queue, so we try again later.
				go func(idx uint32) {
					ledgerIndexes <- idx
				}(idx)
				continue
			}

			if len(txs) > 0 {
				// Render each ledger as a table.
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns) // last parameter flags could include tabwriter.Debug
				fmt.Fprintln(w, util.FormatTransactionWithMetaDataHeader())

				// Order output by order within ledger
				order := make([]int, 0)
				for ledgerOrder, _ := range txs {
					order = append(order, int(ledgerOrder))
				}
				sort.Ints(order)
				for _, ledgerOrder := range order {
					tx := txs[uint32(ledgerOrder)]
					fmt.Fprintln(w, util.FormatTransactionWithMetaDataRow(tx))
					//log.Printf("%s - %s, %s sequence %d, %s in ledger %d (%d)",
					//tx.GetHash(), tx.GetType(), tx.GetBase().Account, tx.GetBase().Sequence, tx.MetaData.TransactionResult, idx, ledgerOrder)

					// Show verbose description
					for _, lint := range util.LintTransaction(tx) {
						fmt.Fprintln(w, lint) // ruins table?
					}
				}
				w.Flush()

				//s.ExitNow() // debug
			}

			// Add the next ledger sequence to our queue.
			go func(i uint32) {
				ledgerIndexes <- i + 1
			}(idx)
		}
	}

}
