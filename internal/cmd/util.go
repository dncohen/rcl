package cmd

import (
	"errors"

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
