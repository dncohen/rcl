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

// RCL Helper Tools
//
// RCL is short for Ripple Consensus Ledger.  (Now called "XRP Ledger".)
//
// Install
//
//     go get -u github.com/dncohen/rcl/cmd/...
//
// Quick Tutorial
//
// Set up a temporary working directory.  We'll use testnet for this example:
//
//     mkdir -p /tmp/rcl-altnet
//     cd /tmp/rcl-altnet
//
//
// Generate some test net XRP to play with:
//
//     curl -s -X POST https://faucet.altnet.rippletest.net/accounts | tee testnet-fund-account.json | python -m json.tool
//
//
// Note the output of above includes...
//
//     "address": "ADDRESS",
//     "secret": "SECRET"
//
// Create a key file in format that rcl tools requires.  Copy the SECRET from the above command into:
//
//    rcl-key generate -nickname fund -secret SECRET
//
//  This will write a file ending `.rcl-key`.  Note the output and
//  confirm that the address shown matches the ADDRESS generated by
//  the faucet.  Although the rcl-key operation is `generate`, the
//  `-secret` flag instructs the command to use an existing account.
//  This puts the secret into a format that `rcl-key sign` can use, as
//  we'll see in a moment. The `-nickname fund` argument will allow us
//  to refer to this account as "fund" later.
//
// Next, create a brand new Ripple address and master signing key with
// `rcl-key` tool:
//
//     rcl-key generate -nickname hot
//
// rcl-key generates a new keypair, and writes both address and secret
// to a file.
//
// The generated *address* (with nickname `hot`) does not become an
// *account* on the test net until it is funded with enough XRP to
// meet the reserve requirement.
//
// Before we use our wallets, its a good practice to make a paper
// backup of the secret keys.  While not strictly speaking a
// requirement when using the testnet, RCL tools expects this will all
// keys.  So let's go through the motions.
//
//    rcl-key backup *.rcl-key
//
// The `rcl-key backup` operation walks you through the secrets,
// giving you a chance to copy each one to paper.  Don't bother for
// test accounts, just press [return] a few times and you'll see how
// `rcl-key backup` works.
//
// As you make a paper backup of each key, the tool writes a
// configuration file with the address, but not the secret, of each
// key.  At this point your working directory should have files
// "fund.cfg", "hot.cfg" and two .rcl-key files with secrets.
//
// In order to create our first transaction, the `rcl-tx` tool must
// communicate with a rippled server. We're going to use the testnet.
// To do so, create a configuration file:
//
//    echo "rippled=wss://s.altnet.rippletest.net:51233" > testnet.cfg
//
// (RCL tools will inspect all the *.cfg files in a given
// configuration directory.  So when we run `rcl-tx`, it will read
// from testnet.cfg as well as fund.cfg and hot.cfg.)
//
// Construct a transaction to send the required XRP to the new address
// with the `rcl-tx send` subcommand:
//
//     rcl-tx -config . -as fund send hot 100/XRP
//
// The output is an unsigned transaction, encoded in JSON format.  The
// unsigned transaction shows us what will be sent to the XRP ledger;
// however, it cannot be sent until it is signed and submitted.
//
// RCL tools uses a "pipeline" to first compose transactions, then
// sign, then submit.  We just saw the first step, composing.  Here's
// how to run the tools in a pipeline, to complete the transaction:
//
//     rcl-tx -config . -as fund send hot 100/XRP | rcl-key sign | rcl-tx -config . submit
//
// The last command, `rcl-tx submit` could take several seconds to
// complete, as it awaits the final status from the XRP ledger
// network.  With a little luck, you should see "tesSUCCESS" in the
// output.
//
// Finally, let's confirm the hot wallet has received some XRP:
//
//    rcl-account -config . -show hot
//
// This should show the 100 XRP that we just sent.
//
//
// Configuration
//
// Most commands require configuration, which is loaded from all `*.cfg` files in a directory.  The default directory is $HOME/.config/rcl/.
//
// Here's an example, save to  i.e. "$HOME/.config/rcl/rcl.cfg":
//
//
//     # Replace this rippled URL with your own trusted rippled
//     # Replace with wss://s.altnet.rippletest.net:51233, for the TEST NET
//     rippled=wss://s1.ripple.com:51233
//
//     # This creates a nickname, `bitstamp-usd` for the Bitstamp issuing address.
//     # optional tag will be used when sending to this address, replace the example below wih your own!
//     [bitstamp-usd]
// 	     address=rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B
// 	     # Put your destination tag below. Remove the "#", which starts a comment.
// 	     #tag=99999999
//
//     [bitstamp-xrp]
// 	     address=rDsbeomae4FXwgQTJp9Rs64Qg9vDiTCdBv
// 	     #tag=99999999
//
//     # deposit XRP to binance
//     [binance]
// 	     address=rEb8TK3gBgk5auZkwc6sHnwrGVJH8DuaLh
// 	     #tag=99999999
//
//     # deposit XRP to sfox
//     [sfox]
// 	     address=rHXeKgsvbrTpq1ii3CeBNrnbUfroi24fT7
// 	     #tag=99999999
//
//     # Add nicknames for your own accounts...
//
package rcl

// Use `go get src.d10.dev/dumbdown` to fetch dumbdown tool.
//go:generate sh -c "go doc | dumbdown > README.md"
//go:generate sh -c "go doc cmd/rcl-account | dumbdown >> README.md"
//go:generate sh -c "go doc cmd/rcl-tx | dumbdown >> README.md"
//go:generate sh -c "go doc cmd/rcl-data | dumbdown >> README.md"
//go:generate sh -c "go doc cmd/rcl-key | dumbdown >> README.md"
