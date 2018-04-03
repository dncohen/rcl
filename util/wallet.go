package util

import (
	"log"

	"github.com/dncohen/rcl/rpc"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/rubblelabs/ripple/websockets"
)

type Wallet struct {
	signKey        Keypair
	Address        string // deprecated, delete once replace with account.
	Account        *data.Account
	Sequence       uint32      // if unknown will be 0
	LedgerSequence uint32      // 0 if unknown, otherwise when account_info returned data.
	Balance        *data.Value // XRP Balance as of LastLedger
	secret         string
	info           *rpc.AccountInfoResult
}

func NewWallet(options ...func(*Wallet) error) (*Wallet, error) {
	wallet := &Wallet{}
	for _, option := range options {
		err := option(wallet)
		if err != nil {
			return wallet, err
		}
	}
	return wallet, nil
}

func SetSecret(secret string) func(*Wallet) error {
	return func(w *Wallet) error {
		var err error
		w.signKey, err = NewEcdsaFromSecret(secret)
		if err != nil {
			return errors.Wrapf(err, "Failed to derive address from secret.")
		}

		// The secret and address go together if secret is master key.
		// If secret is a regular key, address can be different.
		if w.Address == "" {
			w.Address = w.signKey.Address
			acct, err := data.NewAccountFromAddress(w.Address)
			if err != nil {
				return err
			}
			w.Account = acct
		}
		return nil
	}
}

func SetAddress(address string) func(*Wallet) error {
	return func(w *Wallet) error {
		if address != "" {
			acct, err := data.NewAccountFromAddress(address)
			if err != nil {
				return err
			}
			w.Address = address
			w.Account = acct
			log.Printf("wallet address %s, account %s", w.Address, w.Account)
		}
		return nil
	}
}

func (wallet *Wallet) GetAccountInfo(remote *websockets.Remote) error {
	result, err := remote.AccountInfo(*wallet.Account)
	if err != nil {
		return err
	}
	wallet.Sequence = *result.AccountData.Sequence
	wallet.LedgerSequence = result.LedgerSequence // Useful for setting LastLedgerSequence
	wallet.Balance = result.AccountData.Balance   // Useful for knowing if a wallet is funded

	return err
}

// Deprecated in favor of websockets.
func (wallet *Wallet) GetAccountInfoRPC(client rpc.Client, ledgerIndex *uint32) error {
	params := rpc.AccountInfoParams{
		Account: wallet.Address,
	}

	if ledgerIndex != nil {
		i := int32(*ledgerIndex)
		params.Ledger_index = &i
	}

	_, err := client.Do("account_info", params, &wallet.info)

	if err != nil {
		return errors.Wrapf(err, "Failed to get account_info for %s.", wallet.Address)
	}
	wallet.Sequence = wallet.info.Account_data.Sequence
	return err

}

// TODO: this modifies the transaction (?) probably no need to return hash and blob.
func (wallet *Wallet) Sign(t data.Transaction) (hash, tx_blob string, err error) {
	return Sign(t, wallet.signKey)
}
