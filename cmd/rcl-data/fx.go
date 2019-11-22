// Copyright (C) 2019  David N. Cohen

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Operation fx
//
// TODO(dnc): documentation
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dncohen/rcl/rippledata"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     fxMain,
		Name:        "fx",
		Syntax:      "fx <amount> <date>...",
		Description: "Determine historic exchange rate.",
	})
}

func fxMain() error {
	cfg, _ := command.Config()

	dataAPI := cfg.Section("").Key("rippledata").MustString("https://data.ripple.com/v2/") // trailing slash needed
	baseAsset := cfg.Section("").Key("base").MustString("USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B")

	// parse args
	err := command.OperationFlagSet.Parse(command.Args()[1:])
	if err != nil {
		return err
	}

	// validate args
	if len(command.OperationFlagSet.Args()) < 2 {
		return errors.New("fx operation expects a date and amount")
	}

	base, err := data.NewAsset(baseAsset)
	if err != nil {
		command.Check(fmt.Errorf("bad asset (%q): %w", baseAsset, err))
	}

	client, err := rippledata.NewClient(dataAPI) // trailing slash matters!
	if err != nil {
		command.Check(fmt.Errorf("Failed to connect to RCL DataAPI (%q): %w", dataAPI, err)) // exits
	}

	command.V(1).Infof("connected to data API (%q)", dataAPI)

	args := command.OperationFlagSet.Args()

	// first argument is amount
	amount, err := data.NewAmount(args[0])
	if err != nil {
		command.Check(fmt.Errorf("bad amount (%q): %w", args[0], err))
	}

	for _, arg := range args[1:] { // for each date arg
		s := strings.ReplaceAll(arg, "/", "-")            // permit 2006/01/02 or 2006-01-02 on command line
		date, err := time.Parse("2006-01-02 15:04:05", s) // date format preferred
		if err != nil {
			date, err = time.Parse("2006-01-02", s) // another acceptable format
			if err != nil {
				command.Check(fmt.Errorf("bad date: %q", args[0]))
			}
		}

		// TODO(dnc): goroutines for performance
		normalized, err := client.Normalize(*amount, *base, date)

		// output format compatible with ledger-cli
		// https://www.ledger-cli.org/3.0/doc/ledger3.html#Commodity-price-histories
		fmt.Printf("P %s %s %s %s",
			date.Format("2006/01/02 15:04:05"),
			amount.Currency,
			normalized.Rate, base.Currency,
		)
		fmt.Printf(" ; %s %s @@ %s %s\n",
			normalized.Amount, amount.Currency,
			normalized.Converted, base.Currency,
		)
	}

	return nil
}
