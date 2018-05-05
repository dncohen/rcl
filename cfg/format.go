package cfg

import (
	"fmt"

	"github.com/rubblelabs/ripple/data"
)

// Return account's nickname, if any.  Otherwise return account's address.
// Format helper lives in config because this is where nicknames are known.
func (config Config) FormatAccountName(account data.Account) string {
	nick, ok := config.GetAccountNickname(account)
	if ok {
		return nick
	} else {
		return account.String()
	}
}

func (config Config) FormatAmount(amount data.Amount) string {
	nick, ok := config.GetAccountNickname(amount.Issuer)
	if ok {
		return fmt.Sprintf("%s/%s/%s", amount.Value, amount.Currency, nick)
	} else {
		return amount.String()
	}
}
