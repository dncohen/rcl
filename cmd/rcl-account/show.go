package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"golang.org/x/sync/errgroup"

	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

func (s *State) show(args ...string) {
	const help = `

Show current settings and balances for accounts on the Ripple Consensus Ledger.

`

	// subcommand-specific flags
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	fs.Int("ledger", 0, "Ledger sequence number to show. Defaults to most recent.")

	s.ParseFlags(fs, args, help, "show [-ledger=<int>]")

	s.showCommand(fs)
}

func (s *State) showCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " show: ")

	rippled := config.Section("").Key("rippled").String()
	if rippled == "" {
		s.Exitf("rippled websocket address not found in configuration file. Exiting.")
	}

	accounts, err := accountsFromArgs(fs.Args())
	if err != nil {
		s.Exit(err)
	}
	if len(accounts) == 0 {
		log.Println("No accounts specified")
		s.ExitNow()
	}
	//log.Printf("Showing %d accounts", len(accounts))

	var ledger interface{} // fast and loose type definition.  Thanks, JSON.
	ledgerArg := intFlag(fs, "ledger")
	if ledgerArg == 0 {
		ledger = "validated"
	} else {

		// TODO remote.AccountInfo does not yet support this.
		s.Exitf("Currently only supporting 'validated' ledger.")

		ledger = uint32(ledgerArg)
		// TODO check history includes ledger
	}

	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		s.Exitf("Failed to connect to %s: %s", rippled, err)
	}

	// prepare to store data
	linesResults := make(map[*data.Account]*websockets.AccountLinesResult)
	accountResults := make(map[*data.Account]*websockets.AccountInfoResult)
	offerResults := make(map[*data.Account]*websockets.AccountOffersResult)

	g := new(errgroup.Group)

	for _, acct := range accounts {
		acct := acct // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			// TODO handle results with marker!
			result, err := remote.AccountLines(*acct, ledger)
			if err != nil {
				log.Printf("account_lines failed for %s (at ledger %s): %s", acct, ledger, err)
				return err
			} else {
				linesResults[acct] = result
				return nil
			}
		})

		g.Go(func() error {
			result, err := remote.AccountInfo(*acct)
			if err != nil {
				log.Printf("account_info failed for %s: %s", acct, err)
				return err
			} else {
				accountResults[acct] = result
				return nil
			}
		})

		g.Go(func() error {
			result, err := remote.AccountOffers(*acct, ledger)
			if err != nil {
				log.Printf("account_offers failed for %s: %s", acct, err)
				return err
			} else {
				//q.Q(result) // debug
				offerResults[acct] = result
				return nil
			}
		})
	}
	// Wait for all requests to complete
	err = g.Wait()
	if err != nil {
		log.Println(err) // TODO handle better
	}

	// To render peer limit as negative number.
	minusOne, err := data.NewValue("-1", false)
	if err != nil {
		log.Panic(err)
	}

	for key, accountResult := range accountResults {
		account := accountResult.AccountData.Account

		table := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
		fmt.Fprintln(table, "Account\t XRP\t Sequence\t Owner Count\t Ledger Index\t")
		fmt.Fprintf(table, "%s\t %s\t %d\t %d\t %d\t\n",
			account,
			accountResult.AccountData.Balance,
			*accountResult.AccountData.Sequence,
			*accountResult.AccountData.OwnerCount,
			accountResult.LedgerSequence,
		)
		table.Flush()
		fmt.Println("") // blank line

		table = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns)
		fmt.Fprintln(table, "Balances\t Amount\t Currency/Issuer\t Min\t Max\t rippling\t quality\t")
		fmt.Fprintf(table, "%s\t %s\t %s\t\t\t\t\t\t\n", account, accountResult.AccountData.Balance, "XRP")
		for _, line := range linesResults[key].Lines {
			peerLimit, err := line.LimitPeer.Multiply(*minusOne)

			if err != nil {
				log.Panic(err)
			}
			fmt.Fprintf(table, "%s\t %s\t %s/%s\t %s\t %s\t %s\t %s\t\n", account, line.Balance, line.Currency, line.Account, peerLimit, line.Limit, formatRipple(line), formatQuality(line))
			//q.Q(line)
		}
		table.Flush()
		fmt.Println("") // blank line
	}

	// Render all books
	type mappedOffer struct {
		offer   data.AccountOffer
		account data.Account
	}
	byBook := make(map[string][][]mappedOffer)
	for _, account := range accounts {
		if offerResults[account] == nil {
			continue
		}
		book := ""
		bidOrAsk := 0
		for _, offer := range offerResults[account].Offers {
			// Choose a base for this order book.
			bookOption1 := fmt.Sprintf("%s / %s", offer.TakerPays.Asset(), offer.TakerGets.Asset())
			bookOption2 := fmt.Sprintf("%s / %s", offer.TakerGets.Asset(), offer.TakerPays.Asset())

			// Assume "XRP" will be last when sorting.  So XRP will be the base i.e. XRP/USD.
			if strings.Compare(bookOption1, bookOption2) == -1 {
				book = bookOption2
				bidOrAsk = 1
			} else {
				book = bookOption1
				bidOrAsk = 0
			}

			byType, ok := byBook[book]
			if !ok {
				bids := make([]mappedOffer, 0)
				asks := make([]mappedOffer, 0)
				byType = [][]mappedOffer{bids, asks}
				byBook[book] = byType
			}

			byBook[book][bidOrAsk] = append(byType[bidOrAsk], mappedOffer{
				offer:   offer,
				account: *account,
			})
		}
	}

	for bookName, book := range byBook {
		fmt.Println(bookName)

		// sort offers by quality
		for bidOrAsk, _ := range book {
			sort.Slice(book[bidOrAsk], func(i, j int) bool {
				return book[bidOrAsk][i].offer.Quality.Less(book[bidOrAsk][j].offer.Quality.Value)
			})
		}
		table := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns)
		// TODO add currencies to header
		fmt.Fprintln(table, "Bid by / Sequence\t TakerPays\t Price\t Price\t TakerGets\t Ask by / Sequence")
		row := 0
		for row < len(book[0]) || row < len(book[1]) {
			if row < len(book[0]) {
				offer := book[0][row]
				price := offer.offer.TakerGets.Ratio(offer.offer.TakerPays)
				fmt.Fprintf(table, "%s / %d\t %s\t %.4f\t", offer.account, offer.offer.Sequence, offer.offer.TakerPays, price.Float())
			} else {
				fmt.Fprintf(table, "n/a \t \t \t")
			}

			if row < len(book[1]) {
				offer := book[1][row]
				price := offer.offer.TakerPays.Ratio(offer.offer.TakerGets)
				fmt.Fprintf(table, "%.4f\t %s\t %s / %d\t\n", price.Float(), offer.offer.TakerGets, offer.account, offer.offer.Sequence)
			} else {
				fmt.Fprintf(table, "\t \t n/a\t\n")
			}
			row++
		}
		table.Flush()
		fmt.Println("") // blank line
	}

	// devel
	if true {
		return
	}
	// Render all offers...
	// sort all offers into human-readable order
	type mapped struct {
		offer   data.AccountOffer
		account data.Account
	}
	byKey := make(map[string]mapped)
	for _, account := range accounts {

		if offerResults[account] == nil {
			// Account not found
			continue
		}

		for _, offer := range offerResults[account].Offers {
			// sortable key
			key1 := fmt.Sprintf("%s/%s/bid/%s", offer.TakerPays.Asset(), offer.TakerGets.Asset(), offer.Quality) // Arbitrarily call one side the bid
			key2 := fmt.Sprintf("%s/%s/ask/%s", offer.TakerGets.Asset(), offer.TakerPays.Asset(), offer.Quality) // the other side is ask.
			// choose key so that bids and asks are next to each other in final ordering
			key := key1
			if strings.Compare(key1, key2) == -1 {
				key = key2
			}
			byKey[key] = mapped{
				offer:   offer,
				account: *account,
			}
		}
	}

	if len(byKey) > 0 {
		table := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns)
		fmt.Fprintln(table, "Offers\t Sequence\t TakerGets\t TakerPays\t Price\t")

		allKeys := make([]string, 0, len(byKey))
		for k, _ := range byKey {
			allKeys = append(allKeys, k)
		}
		sort.Strings(allKeys)
		for _, k := range allKeys {
			v := byKey[k]
			price := v.offer.TakerPays.Ratio(v.offer.TakerGets)
			fmt.Fprintf(table, "%s\t %d\t %s\t %s\t %s\n", v.account, v.offer.Sequence, v.offer.TakerGets, v.offer.TakerPays, price)
		}
		table.Flush()
		fmt.Println("") // blank line
	}

}

func formatRipple(line data.AccountLine) string {
	if line.NoRipple && line.NoRipplePeer {
		return "none"
	}
	if line.NoRipple && !line.NoRipplePeer {
		return "peer"
	}
	if !line.NoRipple && line.NoRipplePeer {
		return "YES"
	}
	if !line.NoRipple && !line.NoRipplePeer {
		return "BOTH"
	}
	return ""
}

func formatQuality(line data.AccountLine) string {
	if line.QualityIn == 0 && line.QualityOut == 0 {
		return ""
	}
	return "TODO"

}
