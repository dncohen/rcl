# RCL Helper Libraries and Commands

*RCL* is short for Ripple Consensus Ledger.

## Quick Start for First Time Users

Go get it:

```
go get -u github.com/dncohen/rcl/cmd/...
```

Set up a working directory:

```
mkdir -p /tmp/rcl-altnet
cd /tmp/rcl-altnet
```

Generate some test net XRP to play with:

```
curl -s -X POST https://faucet.altnet.rippletest.net/accounts | tee testnet-fund-account.json | python -m json.tool
```

Note the output of above includes...

```
    "address": "<ADDRESS>",
    "secret": "<SECRET>"
```

Copy `<ADDRESS>` and `<SECRET>` and replace into the following.


Configure rcl tools to use the altnet account:

```
echo -e "rippled=wss://s.altnet.rippletest.net:51233" > altnet.cfg
echo -e "[<ADDRESS>]\n\tsecret=<SECRET>\n\tnickname=fund" >> altnet.cfg
```

(You've created a nickname `fund` for the testnet account with 10000 XRP.)

Inspect your account with `rcl-account` tool:

```
rcl-account show fund
```

Create a new Ripple address and master signing key with `rcl-key` tool:

```
rcl-key generate -nickname hot
```

The generated *address* (nickname `hot`) does not become an *account*
on the test net until it is funded with enough XRP to meet the reserve
requirement.

Construct a transaction to send the required XRP to the new address with the `rcl-tx send` subcommand:

```
rcl-tx -as fund send hot 100/XRP > /dev/null
```

Note that `rcl-tx` logs the transaction details, but it does *not* sign
or submit the transaction.  Rather, it *pipes* the transaction to
`stdout` (`/dev/null` in above example).

Here's how to use a *pipeline* to *construct* the tranaction, and *sign* it, and *save* to a file, and *submit* it:

```
rcl-tx -as fund send hot 100/XRP | rcl-key sign | rcl-tx save | rcl-tx submit
```

You should see `tesSUCCESS` in the output from `rcl-tx submit` (after
verbose log output).

Finally check that `hot` account has the balance of `100/XRP`:

```
rcl-account show hot
```



## install and configure

```
go get github.com/dncohen/rcl/cmd/...
```

Most commands require configuration, which is loaded from all `*.cfg` files in a directory.  The default location for configuration files is the current working directory.

Here's an example, save to  `./rcl.cfg`:

```
# Replace this rippled URL with your own trusted rippled
# Replace with wss://s.altnet.rippletest.net:51233, for the TEST NET
rippled=wss://s1.ripple.com:51233

# This creates a nickname, `bitstamp-usd` for the Bitstamp issuing address.
# optional tag will be used when sending to this address, replace the example below wih your own!
[bitstamp-usd]
	address=rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B
	#tag=99999999

[bitstamp-xrp]
	address=rDsbeomae4FXwgQTJp9Rs64Qg9vDiTCdBv
	#tag=99999999

# deposit XRP to binance
[binance]
	address=rEb8TK3gBgk5auZkwc6sHnwrGVJH8DuaLh
	#tag=99999999

# deposit XRP to sfox
[sfox]
	address=rHXeKgsvbrTpq1ii3CeBNrnbUfroi24fT7
	#tag=99999999

# Add nicknames for your own accounts...

```

## develop

In the directory of each command, `go run *.go` will build local files. For example, to run local code for the `rcl-account` command...

```
cd cmd/rcl-account
go run *.go show rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B
```

