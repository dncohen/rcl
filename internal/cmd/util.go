package cmd

import (
	"errors"
	"strings"

	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
	"src.d10.dev/command/config"
)

func Rippled() (string, error) {
	dfault := "wss://s1.ripple.com:51233"
	cfg, err := command.Config()
	if err != nil {
		if errors.Is(err, config.ConfigNotFound) {
			err = nil
		}
		return dfault, err
	}
	rippled := cfg.Section("").Key("rippled").MustString(dfault)
	if rippled == "" {
		return rippled, errors.New("rippled websocket address not found in configuration file")
	}
	return rippled, nil
}

func DataAPI() (string, error) {
	dfault := "https://data.ripple.com/v2/" // trailing slash needed
	cfg, err := command.Config()
	if err != nil {
		if errors.Is(err, config.ConfigNotFound) {
			err = nil
		}
		return dfault, err
	}
	val := cfg.Section("").Key("data").MustString(dfault)
	if val == "" {
		return val, errors.New("data api address not found in configuration file")
	}
	return val, nil
}

func AmountFromArg(arg string) (*data.Amount, error) {
	amt, err := data.NewAmount(arg)
	if err != nil {
		// didn't parse, perhaps the issuer is a nickname
		parts := strings.Split(arg, "/") // i.e. 1/USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B or /1/USD/bitstamp
		if len(parts) == 3 && parts[2] != "" {
			acctArg, err := ParseAccountArg([]string{parts[2]})
			if err != nil {
				return nil, err
			}

			// replace nickname in amount
			amt, err = data.NewAmount(parts[0] + "/" + parts[1] + "/" + acctArg[0].Account.String())
			return amt, err
		}
	}
	return amt, err
}
