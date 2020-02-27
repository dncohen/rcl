package util

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/crypto"
	"github.com/rubblelabs/ripple/data"
	"src.d10.dev/command"
)

const (
	Primary = "primary"
	Issuer  = "issuer"
	Trader  = "trader"
)

type Keypair struct {
	Address string
	secret  string
	KeyType data.KeyType

	// Use rubblelabs library for signing.
	key      crypto.Key
	sequence *uint32
}

// Returns public key in hexidecimal.
func (kp Keypair) PublicKey() data.PublicKey {
	bytes := kp.key.Public(kp.sequence)
	var pubkey data.PublicKey
	n := copy(pubkey[:], bytes)
	if n != len(pubkey) || n != len(bytes) {
		log.Panicf("keypair.PublicKey got %d, expected %d (%d)", n, len(pubkey), len(bytes))
	}
	return pubkey
	//return hex.EncodeToString(kp.key.Public(kp.sequence))
}

// Deprecating RPC in favor of websockets...
func GetRippledUrl() (string, error) {
	rippled := os.Getenv("RIPPLED")
	if rippled == "" {
		return rippled, errors.New("RIPPLED not defined.")
	}
	return rippled, nil
}

func GetRippledWSS() (string, error) {
	rippled := os.Getenv("RIPPLED_WSS")
	if rippled == "" {
		return rippled, errors.New("RIPPLED_WSS not defined.")
	}
	return rippled, nil
}

func GetPrimaryAccount() (*data.Account, error) {
	primaryAddress := os.Getenv("RIPPLE_ADDRESS")
	if primaryAddress == "" {
		// Not set explicitly, try to derive from secret.
		keypair, err := GetPrimaryKeypair()
		if err == nil {
			primaryAddress = keypair.Address
		} else {
			return nil, errors.New("$RIPPLE_ADDRESS not found.")
		}
	}

	return data.NewAccountFromAddress(primaryAddress)
}

func GetPrimaryKeypair() (Keypair, error) {
	primarySecret := os.Getenv("RIPPLE_SECRET")
	if primarySecret == "" {
		return Keypair{}, errors.New("$RIPPLE_SECRET not found.")
	}
	return NewEcdsaFromSecret(primarySecret)
}

func NewEcdsaFromSecret(secret string) (Keypair, error) {
	var addrseq uint32
	pair := Keypair{
		secret:   secret,
		sequence: &addrseq,
		KeyType:  data.ECDSA,
	}

	seed, err := data.NewSeedFromAddress(secret) // Note `Address` is probably a typo.
	if err != nil {
		return pair, err
	}

	pair.Address = seed.AccountId(pair.KeyType, pair.sequence).String()

	pair.key = seed.Key(pair.KeyType)

	return pair, nil
}

func (pair Keypair) Sign(tx data.Transaction) error {
	return data.Sign(tx, pair.key, pair.sequence)
}

// Helpers to submit a rubblelabs data.Transaction.

// Function Sign adds signature to transaction and returns tx_blob suitable for submitting to network.
func Sign(t data.Transaction, keypair Keypair) (hash, tx_blob string, err error) {
	err = keypair.Sign(t)
	if err != nil {
		return
	}
	_, raw, err := data.Raw(t)
	if err != nil {
		return
	}

	// debug.  confirm raw can be decoded.
	//_, err = data.ReadTransaction(bytes.NewReader(raw))
	//if err != nil {
	//	q.Q(t)
	//  return errors.Wrap(err, "tx blog read test")
	//}

	tx_blob = fmt.Sprintf("%X", raw)
	hash = t.GetHash().String()
	return
}

// Function ParseCompleteLedgers accepts a complete_ledgers string and
// returns the min and max ledgers of the most recent continuous chunk
// of ledger history available.
func ParseCompleteLedgers(v string) (uint32, uint32, error) {
	var min, max int64
	var err error

	// Gaps are represented by comma.
	chunks := strings.Split(v, ",")
	// Continuous chunk represted by dash.  We ignore all but the last chunk.
	bounds := strings.Split(chunks[len(chunks)-1], "-")

	// confirm we have valid range
	if len(bounds) != 2 {
		return 0, 0, errors.Errorf("Failed to parse ledger history: %s", v)
	}
	min, err = strconv.ParseInt(bounds[0], 10, 32)
	if err == nil {
		max, err = strconv.ParseInt(bounds[1], 10, 32)
		if err == nil {
			return uint32(min), uint32(max), nil
		}
	}

	return 0, 0, errors.Wrapf(err, "Failed to parse ledger history: %s", v)

}

// Print a numeric value, avoiding scientific notation.
func FormatValue(v data.Value) string {
	// unfortunately rubblelabs does not export data.Value:isScientific,
	// so we must manipulate strings
	str := v.String()
	if strings.Index(str, "e") != -1 {
		// simply using fmt.Sprintf("%f", v.Float()) produces, for example "0.000000"

		rat := v.Rat()
		// debug
		command.V(1).Infof("formatValue: converting scientific notation from %q to %q", v.String(), rat.FloatString(16))
		str = fmt.Sprintf("%s", rat.FloatString(16)) // TODO(dnc): proper decimal precision?
	}
	return str
}
