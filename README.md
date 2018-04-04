# RCL Helper Libraries and Commands

## Quick Start for First Timers

Go get it:

```
go get -u github.com/dncohen/rcl/cmd/...
```

Set up a sandbox:

```
mkdir -p /tmp/rcl-altnet
cd /tmp/rcl-altnet
```

Generate some test net XRP to play with:

```
curl -s -X POST https://faucet.altnet.rippletest.net/accounts | tee testnet-fund-account.json | python -m json.tool
```

Note the output has

```
    "address": "<ADDRESS>",
    "secret": "<SECRET>"
```

Copy <ADDRESS> and <SECRET> and replace into the following...


Configure `rcl` tools to use the altnet:

```
echo -e "rippled=wss://s.altnet.rippletest.net:51233" > altnet.cfg
echo -e "[fund]\n\taddress=<ADDRESS>\n\tsecret=<SECRET>" >> altnet.cfg
```

(You've created an alias `fund` for the testnet account with 10000 XRP.)

Use the `rcl-account` command to inspect your account:

```
rcl-account show fund
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

Or, install your local modifications with...

```
go get cmd/...
```

... then run `rcl-account` as you normally would.


