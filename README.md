# RCL Helper Libraries and Commands

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


