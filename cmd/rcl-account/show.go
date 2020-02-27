// Copyright (C) 2018-2020  David N. Cohen
// This file is part of github.com/dncohen/rcl
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Command RCL-account - Operation Show
//
//    rcl-account show <address> [<address> ...]
//
// Prints in human-readable format the balances of one or more accounts.
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"golang.org/x/sync/errgroup"
	"src.d10.dev/command"

	"github.com/dncohen/rcl/internal/cmd"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opShow,
		Name:        "show",
		Syntax:      "show [-ledger=<int>] <account> [...]",
		Description: `Show current account balances.`,
	})
}

func opShow() error {

	ledgerFlag := command.OperationFlagSet.Int("ledger", -1, "ledger sequence number to show; use -1 for most recent.")

	err := command.ParseOperationFlagSet()
	command.CheckUsage(err)

	rippled, err := cmd.Rippled()
	command.Check(err)

	// accept addresses or nicknames as arguments
	account, err := cmd.ParseAccountArg(command.OperationFlagSet.Args())
	command.Check(err)

	if len(account) == 0 {
		command.CheckUsage(errors.New("expected one or more addresses"))
	}
	//log.Printf("Showing %d accounts", len(accounts))

	var ledger interface{} // fast and loose type definition, brought to you by JSON

	if *ledgerFlag == -1 {
		ledger = "validated"
	} else {

		// TODO remote.AccountInfo does not yet support this.
		command.Check(errors.New("currently only supporting 'validated' ledger"))

		ledger = uint32(*ledgerFlag)
		// TODO check history includes ledger
	}

	remote, err := websockets.NewRemote(rippled)
	if err != nil {
		command.Check(fmt.Errorf("Failed to connect to %s: %s", rippled, err))
	}

	// prepare to store data
	mutex := &sync.Mutex{}
	linesResults := make(map[data.Account]*websockets.AccountLinesResult)
	accountResults := make(map[data.Account]*websockets.AccountInfoResult)
	offerResults := make(map[data.Account]*websockets.AccountOffersResult)

	g := new(errgroup.Group)

	for _, acct := range account {
		acct := acct // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			// TODO handle results with marker!
			result, err := remote.AccountLines(acct.Account, ledger)
			if err != nil {
				command.Errorf("account_lines failed for %s (at ledger %s): %s", acct, ledger, err)
				return err
			} else {
				mutex.Lock()
				defer mutex.Unlock()

				linesResults[acct.Account] = result
				return nil
			}
		})

		g.Go(func() error {
			result, err := remote.AccountInfo(acct.Account)
			if err != nil {
				command.Errorf("account_info failed for %s: %s", acct, err)
				return err
			} else {
				mutex.Lock()
				defer mutex.Unlock()

				accountResults[acct.Account] = result
				return nil
			}
		})

		g.Go(func() error {
			result, err := remote.AccountOffers(acct.Account, ledger)
			if err != nil {
				command.Errorf("account_offers failed for %s: %s", acct, err)
				return err
			} else {
				mutex.Lock()
				defer mutex.Unlock()

				offerResults[acct.Account] = result
				return nil
			}
		})
	}
	// Wait for all requests to complete
	err = g.Wait()
	command.Check(err)

	// To render peer limit as negative number.
	minusOne, err := data.NewValue("-1", false)
	command.Check(err)

	for key, accountResult := range accountResults {
		account := accountResult.AccountData.Account

		table := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
		fmt.Fprintln(table, "Account\t XRP\t Sequence\t Owner Count\t Ledger Index\t")
		fmt.Fprintf(table, "%s\t %s\t %d\t %d\t %d\t\n",
			cmd.FormatAccount(*account, nil),
			accountResult.AccountData.Balance,
			*accountResult.AccountData.Sequence,
			*accountResult.AccountData.OwnerCount,
			accountResult.LedgerSequence,
		)
		table.Flush()
		fmt.Println("") // blank line

		table = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns|tabwriter.Debug)
		fmt.Fprintln(table, "Balances\t Amount\t Currency/Issuer\t Min\t Max\t rippling\t quality\t")
		fmt.Fprintf(table, "%s\t %s\t %s\t\t\t\t\t\n", cmd.FormatAccount(*account, nil), accountResult.AccountData.Balance, "XRP")
		for _, line := range linesResults[key].Lines {
			peerLimit, err := line.LimitPeer.Multiply(*minusOne)
			command.Check(err)

			fmt.Fprintf(table, "%s\t %s\t %s/%s\t %s\t %s\t %s\t %s\t\n", cmd.FormatAccount(*account, nil), line.Balance, line.Currency, line.Account, peerLimit, line.Limit, formatRipple(line), formatQuality(line))
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
	for _, acct := range account {
		if offerResults[acct.Account] == nil {
			continue
		}
		book := ""
		bidOrAsk := 0
		for _, offer := range offerResults[acct.Account].Offers {
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
				account: acct.Account,
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
		table := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.DiscardEmptyColumns|tabwriter.Debug)
		// TODO add currencies to header
		fmt.Fprintln(table, "Bid by (Sequence)\t TakerPays\t Price\t Price\t TakerGets\t Ask by (Sequence)")
		row := 0
		for row < len(book[0]) || row < len(book[1]) {
			if row < len(book[0]) {
				offer := book[0][row]
				price := offer.offer.TakerGets.Ratio(offer.offer.TakerPays)
				fmt.Fprintf(table, "%s (%d)\t %s\t %.4f\t", cmd.FormatAccount(offer.account, nil), offer.offer.Sequence, offer.offer.TakerPays, price.Float())
			} else {
				fmt.Fprintf(table, "n/a \t \t \t")
			}

			if row < len(book[1]) {
				offer := book[1][row]
				price := offer.offer.TakerPays.Ratio(offer.offer.TakerGets)
				fmt.Fprintf(table, "%.4f\t %s\t %s (%d)\t\n", price.Float(), offer.offer.TakerGets, cmd.FormatAccount(offer.account, nil), offer.offer.Sequence)
			} else {
				fmt.Fprintf(table, "\t \t n/a\t\n")
			}
			row++
		}
		table.Flush()
		fmt.Println("") // blank line
	}
	return nil
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
