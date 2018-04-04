package cfg

// Provides a consistent configuration file format accross all the commands in ../cmd.

// rcl.cfg file example:

/*
# rippled websockets URL.  For testnet, use "wss://s.altnet.rippletest.net:51233"
rippled=wss://s1.ripple.com:51233

# Default account when constructing transactions.
account=rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B

# Other accounts...
[rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B]
	nickname=bitstamp

[rGFuMiw48HdbnrUbkRYuitXTmfrDBNTCnX]
	nickname=bitstamp (hot)

[rDsbeomae4FXwgQTJp9Rs64Qg9vDiTCdBv]
	nickname=bitstamp (xrp)

*/

// rcl-secret.cfg example:

/*

[ecdsa]
rPm4uZoAxbM3f6neoidWr5BMzuDB1ocRjT=sSomethingSecretGoesRightHere
r9miHh8cFcCFaCprknpdFvyYcy95ZqKhun=sSomethingSecretGoesRightHere

*/

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dncohen/rcl/util"
	"github.com/go-ini/ini"
	"github.com/rubblelabs/ripple/data"
)

type Config struct {
	*ini.File
	// map nicknames to their configuration
	accounts map[string]*ini.Section
}

// Helper loads multiple config files
func LooseLoadGlob(pattern string) (Config, error) {
	configFilenames, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	if len(configFilenames) == 0 {
		fmt.Println("No configuration file found.  Create rcl.cfg.")
		os.Exit(2)
	}
	// https://golang.org/doc/faq#convert_slice_of_interface
	configs := make([]interface{}, len(configFilenames))
	for i, v := range configFilenames {
		configs[i] = v
	}

	return LooseLoad(configs[0], configs[1:]...)
}

// Wrapper around https://godoc.org/gopkg.in/go-ini/ini.v1#LooseLoad
func LooseLoad(source interface{}, others ...interface{}) (Config, error) {
	config := Config{}
	var err error

	config.File, err = ini.LooseLoad(source, others...)
	if err != nil {
		fmt.Printf("Failed to load configuration file: %s", err)
		os.Exit(2)
	}
	// Create a mapping of nickname -> account.
	config.accounts = make(map[string]*ini.Section)
	for _, section := range config.Sections() {
		if section.HasKey("nickname") {
			nickname := section.Key("nickname").String()
			address := section.Name()
			_, err := data.NewAccountFromAddress(address)
			if err != nil {
				log.Printf("Bad address %s in [%s]", address, section.Name())
			} else {
				//log.Printf("Nick: %s, Acct: %s", nickname, account) // debug
				config.accounts[nickname] = section
				config.accounts[nickname].NewKey("address", address)
			}
		}
		if section.HasKey("address") {
			nickname := section.Name()
			_, err := data.NewAccountFromAddress(section.Key("address").String())
			if err != nil {
				log.Printf("Bad address [%s] with nickname \"%s\"\n", section.Key("address"), section.Name())
			} else {
				config.accounts[nickname] = section
			}
		}
	}

	return config, err
}

func (config Config) GetRippled() string {
	return config.Section("").Key("rippled").String()
}

func (config Config) GetAccount() string {
	return config.Section("").Key("account").String()
}

func (config Config) GetAccountByNickname(nickname string) (account *data.Account, tag *uint32, ok bool) {
	var err error

	section, ok := config.accounts[nickname]
	if ok {
		account, err = data.NewAccountFromAddress(section.Key("address").String())
		if err != nil {
			log.Printf("Bad address: %s\n", section.Key("address"))
			return account, tag, false
		}

		if section.HasKey("tag") {
			tagUint, err := section.Key("tag").Uint()
			if err != nil {
				log.Printf("config [%s] bad tag \"%s\": %s", section.Name(), section.Key("tag"), err)
				return account, tag, false
			} else {
				// Return *uint32
				tmp := uint32(tagUint)
				tag = &tmp
			}
		}
	}

	return account, tag, ok
}

func (config Config) GetAccountNickname(account data.Account) (string, bool) {
	nick := config.Section(account.String()).Key("nickname").String()
	return nick, nick != ""
}

// TODO support ECDSA and ed25519
func (config Config) GetAccountKeypair(account data.Account) (util.Keypair, error) {
	secret := config.Section("ecdsa").Key(account.String()).String()
	if secret == "" {
		// Top-level section is assumed ECDSA
		secret = config.Section("").Key(account.String()).String()
	}

	if secret == "" {
		return util.Keypair{}, fmt.Errorf("No secret found for %s", account)
	}

	return util.NewEcdsaFromSecret(secret)

}

// Helper to parse a command line argument into a fully-qualifed account.
func (config Config) AccountFromArg(arg string) (*data.Account, *uint32, error) {
	// TODO split arg into address and destination tag
	acct, err := data.NewAccountFromAddress(arg)
	var tag *uint32
	var ok bool
	if err != nil {
		// maybe the arg is a nickname and not an address
		acct, tag, ok = config.GetAccountByNickname(arg)
		if acct == nil || !ok {
			return acct, tag, fmt.Errorf("Not an address: %s", arg)
		}
	}
	return acct, tag, nil
}

func (config Config) AmountFromArg(arg string) (*data.Amount, error) {
	amt, err := data.NewAmount(arg)
	if err != nil {
		parts := strings.Split(arg, "/") // i.e. 1/USD/rvYAfWj5gh67oV6fW32ZzP3Aw4Eubs59B or /1/USD/bitstamp
		log.Println(parts)               // debug
		if len(parts) == 3 && parts[2] != "" {
			acct, _, ok := config.GetAccountByNickname(parts[2])
			if ok {
				amt, err = data.NewAmount(parts[0] + "/" + parts[1] + "/" + acct.String())
			}
		}
	}
	return amt, err
}
