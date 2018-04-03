package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"github.com/rubblelabs/ripple/data"
)

type TxParams struct {
	// hash
	Transaction string `json:"transaction"`
}

// Example responses:

// {"Account":"rMumqWWTVhGAo1Zud8dFDzS9N5RNEDK1JS","Fee":"20","Sequence":4,"SetFlag":2,"SigningPubKey":"03364EA1B298A4BE65CE1C9FD6EA31B273BFE8F07E953378F20D65A0C166CE8AEA","TransactionType":"AccountSet","TxnSignature":"3045022100E5077C44290E511E83738FBB4D6F3283D4F67E731CC6F75AE01170C0CC36CCD2022017CA964FB122F48705E8E6FF81D2D70511C41FFAE11FB096BF78A9C353B198E3","hash":"2F6619C1AC2A482F50810C307753C71CDFB4343F023D7CC52C13FD647A5BBD5E","status":"success"}

// {"Account":"rMWAmTH2Lc3GWJN6AYBXPXTPjcsSve8Zqu","Fee":"20","Sequence":5,"SetFlag":2,"SigningPubKey":"022145995B86AD97B4E63881979CF2E565EF0C35DE2F3B948F1959241C0FD96A99","TransactionType":"AccountSet","TxnSignature":"3045022100B665E18065F5232D81C30F0CBA76C8989B44DA1D0312FCD547558BE81A0FB60302203F0CF377B268396EA99F002F93A1A0B32289BEEA7A1A098E9561446481497C5F","date":551089522,"hash":"C6BFEC290E4B3D3164F0F1D6BB09EFE72C8E23BF474DC5826FEC4A747904D682","inLedger":2061007,"ledger_index":2061007,"meta":{"AffectedNodes":[{"ModifiedNode":{"FinalFields":{"Account":"rMWAmTH2Lc3GWJN6AYBXPXTPjcsSve8Zqu","Balance":"9999999900","Flags":262144,"OwnerCount":0,"Sequence":6},"LedgerEntryType":"AccountRoot","LedgerIndex":"9906C45D56EA6C4DFA5AD490ABB64AEB1F8F5DE763FFED320A8E899F20516DAE","PreviousFields":{"Balance":"9999999920","Sequence":5},"PreviousTxnID":"F6F8D76DF5A63EC043DAE8C8A1948E5E311FED243F4106D6DEE9786ECCBE0F90","PreviousTxnLgrSeq":2060767}}],"TransactionIndex":3,"TransactionResult":"tesSUCCESS"},"status":"success","validated":true}

// account_tx returns (no status or validated)
// "tx": {
//     "Account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//     "Fee": "10",
//     "Flags": 0,
//     "LimitAmount": {
//         "currency": "USD",
//         "issuer": "r3kmLJN5D28dHuH8vZNUZpMC43pEHpaocV",
//         "value": "100"
//     },
//     "Sequence": 2,
//     "SigningPubKey": "02BC8C02199949B15C005B997E7C8594574E9B02BA2D0628902E0532989976CF9D",
//     "TransactionType": "TrustSet",
//     "TxnSignature": "304402200EF81EC32E0DFA9BE376B20AFCA11765ED9FEA04CA8B77C7178DAA699F7F5AFF02202DA484DBD66521AC317D84F7717EC4614E2F5DB743E313E8B48440499CC0DBA4",
//     "date": 413620090,
//     "hash": "002AA492496A1543DBD3680BF8CF21B6D6A078CE4A01D2C1A4B63778033792CE",
//     "inLedger": 195480,
//     "ledger_index": 195480
// },

type MetaData struct {
	AffectedNodes     data.NodeEffects
	TransactionIndex  uint32
	TransactionResult data.TransactionResult
	DeliveredAmount   *data.Amount `json:"delivered_amount,omitempty"`
}

type TxResult struct {
	Result
	data.TxBase

	Ledger_index uint32 `json:"ledger_index,omitempty"` // account_tx returns this

	Meta      *MetaData // "tx" returns this.
	Validated *bool     // "tx" returns this.

	// When unmarshalling, save the raw bytes for further unmarshalling
	// into type-specific structs.
	raw []byte
}

type TxResultXXX struct {
	Result
	Account         string
	Fee             uint64 `json:",string"`
	Sequence        uint32
	TransactionType string
	Hash            string `json:"hash"`
	Flags           *data.TransactionFlag

	Ledger_index uint32 `json:"ledger_index,omitempty"` // account_tx returns this

	Meta      *MetaData // "tx" returns this.
	Validated *bool     // "tx" returns this.

	// When unmarshalling, save the raw bytes for further unmarshalling
	// into type-specific structs.
	raw []byte
}

type TxResultNoRaw TxResult // Same fields, without methods.  Use this when you don't want custom json unmarshalling.

func (tx *TxResult) UnmarshalJSON(data []byte) error {
	tx.raw = data
	a := (*TxResultNoRaw)(tx)
	return json.Unmarshal(data, a)
}

func (tx *TxResult) Raw() []byte {
	return tx.raw
}

func (result *TxResult) String() string {
	return fmt.Sprintf("type: %s account: %s sequence: %d hash: %s ", result.TransactionType, result.Account, result.Sequence, result.Hash)
}

// For verbose debug
func (tx *TxResult) Json() string {
	return string(tx.Raw())
}

func (tx *TxResult) IsValidated() bool {
	return tx.Validated != nil && *tx.Validated
}

func (tx *TxResult) Succeeded() bool {
	return tx.IsValidated() && tx.Meta.TransactionResult.Success()
}

// Decodes a hex memo to string.
// Index i specificies which memo, starting with zero.
// Returns ok == false if there is no such memo.
func (tx *TxResult) Memo(i int) (string, bool) {
	if tx.Memos == nil {
		return "", false
	}
	if len(tx.Memos) <= i {
		return "", false
	}
	decoded, err := hex.DecodeString(tx.Memos[0].Memo.MemoData.String())
	if err != nil {
		log.Panic(err)
	}
	return string(decoded), true
}

// {
//          "Account": "rUf4MCHPbwfefiNWADa9jBBvuBVhR2tq4K",
//          "Amount": "31000000",
//          "Destination": "r9ky9zG7dwMgSyr1BGPsrfU6HR5ScedbYo",
//          "Fee": "20",
//          "Flags": 2147483648,
//          "Sequence": 5,
//          "SigningPubKey": "03DA2E7ED328A9B7B0FB3456E47A41FD2EAB7626839456F172AE5E389AA2B8AB1D",
//          "TransactionType": "Payment",
//          "TxnSignature": "3045022100FFC788444219BE2A867D9CFA35071A645DD15CFAD77F233CCE1454A19402D0210220102F38A5FCAA6E4774A16C7A176C6EBD0755727DF32
//
//          "date": 551548520,
//          "hash": "664DF9C57C9CFEA46886286DB16A7A8CD6D03D8B15DDA26CA1A8AC9656F0F080",
//          "inLedger": 48355,
//          "ledger_index": 48355
//  }

// Payments are a transaction type that we want to inspect more fields than TxResult provides.
type PaymentResultXXX struct {
	TxResultNoRaw

	Amount         data.Amount
	Address        string
	Destination    string
	InvoiceID      *string
	DestinationTag *uint32
}

type PaymentResultXXXX struct {
	Result
	data.Payment

	Ledger_index uint32 `json:"ledger_index,omitempty"` // account_tx returns this

	Meta      *MetaData // "tx" returns this.
	Validated *bool     // "tx" returns this.

}

func (tx TxResult) Payment() (data.Payment, error) {
	payment := data.Payment{}

	err := json.Unmarshal(tx.Raw(), &payment)
	return payment, err
}
