package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/rubblelabs/ripple/data"
)

// {
//     "account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//     "binary": false,
//     "count": false,
//     "descending": false,
//     "forward": false,
//     "ledger_index_max": -1,
//     "ledger_index_min": -1,
//     "limit": 2,
//     "offset": 1
// }

type AccountTxParams struct {
	Account          string          `json:"account"`
	Ledger_index_min uint32          `json:"ledger_index_min"`
	Ledger_index_max uint32          `json:"ledger_index_max"`
	Limit            int             `json:"limit"`
	Forward          bool            `json:"forward"`
	Marker           json.RawMessage `json:"marker"`
}

// {
//     "result": {
//         "account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//         "ledger_index_max": 8696227,
//         "ledger_index_min": 32570,
//         "limit": 2,
//         "offset": 1,
//         "status": "success",
//         "transactions": [
//             {
//                 "meta": {
//                     "AffectedNodes": [
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "Account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//                                     "Balance": "9999999980",
//                                     "Flags": 0,
//                                     "OwnerCount": 2,
//                                     "Sequence": 3
//                                 },
//                                 "LedgerEntryType": "AccountRoot",
//                                 "LedgerIndex": "4F83A2CF7E70F77F79A307E6A472BFC2585B806A70833CCD1C26105BAE0D6E05",
//                                 "PreviousFields": {
//                                     "Balance": "9999999990",
//                                     "OwnerCount": 1,
//                                     "Sequence": 2
//                                 },
//                                 "PreviousTxnID": "389720F6FD8A144F171708F9ECB334D704CBCFEFBCDA152D931AC34FB5F9E32B",
//                                 "PreviousTxnLgrSeq": 95405
//                             }
//                         },
//                         {
//                             "CreatedNode": {
//                                 "LedgerEntryType": "RippleState",
//                                 "LedgerIndex": "718C6D58DD3BBAAAEBFE48B8FBE3C32C9F6F2EBC395233BA95D0057078EE07DB",
//                                 "NewFields": {
//                                     "Balance": {
//                                         "currency": "USD",
//                                         "issuer": "rrrrrrrrrrrrrrrrrrrrBZbvji",
//                                         "value": "0"
//                                     },
//                                     "Flags": 131072,
//                                     "HighLimit": {
//                                         "currency": "USD",
//                                         "issuer": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//                                         "value": "100"
//                                     },
//                                     "LowLimit": {
//                                         "currency": "USD",
//                                         "issuer": "r3kmLJN5D28dHuH8vZNUZpMC43pEHpaocV",
//                                         "value": "0"
//                                     }
//                                 }
//                             }
//                         },
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "Flags": 0,
//                                     "Owner": "r3kmLJN5D28dHuH8vZNUZpMC43pEHpaocV",
//                                     "RootIndex": "77F65EFF930ED7E93C6CC839C421E394D6B1B6A47CEA8A140D63EC9C712F46F5"
//                                 },
//                                 "LedgerEntryType": "DirectoryNode",
//                                 "LedgerIndex": "77F65EFF930ED7E93C6CC839C421E394D6B1B6A47CEA8A140D63EC9C712F46F5"
//                             }
//                         },
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "Account": "r3kmLJN5D28dHuH8vZNUZpMC43pEHpaocV",
//                                     "Balance": "78991384535796",
//                                     "Flags": 0,
//                                     "OwnerCount": 3,
//                                     "Sequence": 188
//                                 },
//                                 "LedgerEntryType": "AccountRoot",
//                                 "LedgerIndex": "B33FDD5CF3445E1A7F2BE9B06336BEBD73A5E3EE885D3EF93F7E3E2992E46F1A",
//                                 "PreviousTxnID": "E9E1988A0F061679E5D14DE77DB0163CE0BBDC00F29E396FFD1DA0366E7D8904",
//                                 "PreviousTxnLgrSeq": 195455
//                             }
//                         },
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "ExchangeRate": "4E11C37937E08000",
//                                     "Flags": 0,
//                                     "RootIndex": "F60ADF645E78B69857D2E4AEC8B7742FEABC8431BD8611D099B428C3E816DF93",
//                                     "TakerGetsCurrency": "0000000000000000000000000000000000000000",
//                                     "TakerGetsIssuer": "0000000000000000000000000000000000000000",
//                                     "TakerPaysCurrency": "0000000000000000000000004254430000000000",
//                                     "TakerPaysIssuer": "5E7B112523F68D2F5E879DB4EAC51C6698A69304"
//                                 },
//                                 "LedgerEntryType": "DirectoryNode",
//                                 "LedgerIndex": "F60ADF645E78B69857D2E4AEC8B7742FEABC8431BD8611D099B428C3E816DF93"
//                             }
//                         }
//                     ],
//                     "TransactionIndex": 0,
//                     "TransactionResult": "tesSUCCESS"
//                 },
//                 "tx": {
//                     "Account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//                     "Fee": "10",
//                     "Flags": 0,
//                     "LimitAmount": {
//                         "currency": "USD",
//                         "issuer": "r3kmLJN5D28dHuH8vZNUZpMC43pEHpaocV",
//                         "value": "100"
//                     },
//                     "Sequence": 2,
//                     "SigningPubKey": "02BC8C02199949B15C005B997E7C8594574E9B02BA2D0628902E0532989976CF9D",
//                     "TransactionType": "TrustSet",
//                     "TxnSignature": "304402200EF81EC32E0DFA9BE376B20AFCA11765ED9FEA04CA8B77C7178DAA699F7F5AFF02202DA484DBD66521AC317D84F7717EC4614E2F5DB743E313E8B48440499CC0DBA4",
//                     "date": 413620090,
//                     "hash": "002AA492496A1543DBD3680BF8CF21B6D6A078CE4A01D2C1A4B63778033792CE",
//                     "inLedger": 195480,
//                     "ledger_index": 195480
//                 },
//                 "validated": true
//             },
//             {
//                 "meta": {
//                     "AffectedNodes": [
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "Account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//                                     "Balance": "9999999970",
//                                     "Flags": 0,
//                                     "OwnerCount": 3,
//                                     "Sequence": 4
//                                 },
//                                 "LedgerEntryType": "AccountRoot",
//                                 "LedgerIndex": "4F83A2CF7E70F77F79A307E6A472BFC2585B806A70833CCD1C26105BAE0D6E05",
//                                 "PreviousFields": {
//                                     "Balance": "9999999980",
//                                     "OwnerCount": 2,
//                                     "Sequence": 3
//                                 },
//                                 "PreviousTxnID": "002AA492496A1543DBD3680BF8CF21B6D6A078CE4A01D2C1A4B63778033792CE",
//                                 "PreviousTxnLgrSeq": 195480
//                             }
//                         },
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "Flags": 0,
//                                     "Owner": "r3PDtZSa5LiYp1Ysn1vMuMzB59RzV3W9QH",
//                                     "RootIndex": "A39F044D860C5B5846AA7E0FAAD44DC8897F0A62B2F628AA073B21B3EC146010"
//                                 },
//                                 "LedgerEntryType": "DirectoryNode",
//                                 "LedgerIndex": "A39F044D860C5B5846AA7E0FAAD44DC8897F0A62B2F628AA073B21B3EC146010"
//                             }
//                         },
//                         {
//                             "ModifiedNode": {
//                                 "LedgerEntryType": "AccountRoot",
//                                 "LedgerIndex": "E0D7BDE68B468FF0B8D948FD865576517DA987569833A05374ADB9A72E870A06",
//                                 "PreviousTxnID": "0222B59280D165D40C464EA75AAD08A4D152C46A38C0625DEECF6EE87FC5B9E1",
//                                 "PreviousTxnLgrSeq": 343555
//                             }
//                         },
//                         {
//                             "CreatedNode": {
//                                 "LedgerEntryType": "RippleState",
//                                 "LedgerIndex": "EA4BF03B4700123CDFFB6EB09DC1D6E28D5CEB7F680FB00FC24BC1C3BB2DB959",
//                                 "NewFields": {
//                                     "Balance": {
//                                         "currency": "USD",
//                                         "issuer": "rrrrrrrrrrrrrrrrrrrrBZbvji",
//                                         "value": "0"
//                                     },
//                                     "Flags": 131072,
//                                     "HighLimit": {
//                                         "currency": "USD",
//                                         "issuer": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//                                         "value": "100"
//                                     },
//                                     "LowLimit": {
//                                         "currency": "USD",
//                                         "issuer": "r3PDtZSa5LiYp1Ysn1vMuMzB59RzV3W9QH",
//                                         "value": "0"
//                                     }
//                                 }
//                             }
//                         },
//                         {
//                             "ModifiedNode": {
//                                 "FinalFields": {
//                                     "ExchangeRate": "4E11C37937E08000",
//                                     "Flags": 0,
//                                     "RootIndex": "F60ADF645E78B69857D2E4AEC8B7742FEABC8431BD8611D099B428C3E816DF93",
//                                     "TakerGetsCurrency": "0000000000000000000000000000000000000000",
//                                     "TakerGetsIssuer": "0000000000000000000000000000000000000000",
//                                     "TakerPaysCurrency": "0000000000000000000000004254430000000000",
//                                     "TakerPaysIssuer": "5E7B112523F68D2F5E879DB4EAC51C6698A69304"
//                                 },
//                                 "LedgerEntryType": "DirectoryNode",
//                                 "LedgerIndex": "F60ADF645E78B69857D2E4AEC8B7742FEABC8431BD8611D099B428C3E816DF93"
//                             }
//                         }
//                     ],
//                     "TransactionIndex": 0,
//                     "TransactionResult": "tesSUCCESS"
//                 },
//                 "tx": {
//                     "Account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//                     "Fee": "10",
//                     "Flags": 0,
//                     "LimitAmount": {
//                         "currency": "USD",
//                         "issuer": "r3PDtZSa5LiYp1Ysn1vMuMzB59RzV3W9QH",
//                         "value": "100"
//                     },
//                     "Sequence": 3,
//                     "SigningPubKey": "02BC8C02199949B15C005B997E7C8594574E9B02BA2D0628902E0532989976CF9D",
//                     "TransactionType": "TrustSet",
//                     "TxnSignature": "3044022058A89552068D1A274EE72BA71363E33E54E6608BC28A84DEC6EE530FC2B5C979022029F4D1EA1237A1F717C5F5EC526E6CFB6DF54C30BADD25EDDE7D2FDBC8F17E34",
//                     "date": 416347560,
//                     "hash": "53354D84BAE8FDFC3F4DA879D984D24B929E7FEB9100D2AD9EFCD2E126BCCDC8",
//                     "inLedger": 343570,
//                     "ledger_index": 343570
//                 },
//                 "validated": true
//             }
//         ],
//         "validated": true
//     }
// }

type AccountTxResult struct {
	Result
	Account string `json:"account"`
	// data.TransactionWithMetaData does not decode correctly.  For example delivered amount is never populated.
	//Transactions []data.TransactionWithMetaData `json:"transactions"`

	// This is useful for seeing the full json returned.
	//Transactions json.RawMessage `json:"transactions"`

	Transactions []TxMeta        `json:"transactions"`
	Marker       json.RawMessage `json:"marker, omitempty"`
}

type TxMeta struct {
	Tx TxResult `json:"tx"` // Note this will not have "validated" or "status"
	//Tx        data.Transaction `json:"tx"` // rubblelabs (better?)
	Meta      TxMetaData `json:"meta"`
	Validated *bool      `json:"validated"`
}

type TxMetaRaw struct {
	Tx        json.RawMessage `json:"tx"`
	Meta      json.RawMessage `json:"meta"`
	Validated *bool           `json:"validated"`
}

// Not sure whether rubblelabs metadata.go needs a fix for delivered amount.
// Should this be deprecated in favor of Metadata in tx.go?
type TxMetaData struct {
	AffectedNodes     data.NodeEffects
	TransactionIndex  uint32
	TransactionResult data.TransactionResult
	DeliveredAmount   *data.Amount `json:"delivered_amount,omitempty"`
}

func (tm *TxMeta) IsValidated() bool {
	return tm.Validated != nil && *tm.Validated
}

func (tm *TxMeta) String() string {
	return fmt.Sprintf("type: %s account: %s sequence: %d status: %s validated: %t hash: %s ", tm.Tx.TransactionType, tm.Tx.Account, tm.Tx.Sequence, tm.Meta.TransactionResult, tm.IsValidated(), tm.Tx.Hash)
}

func (a *TxMeta) Before(b *TxMeta) bool {
	if a.Tx.Ledger_index < b.Tx.Ledger_index {
		return true
	} else if a.Tx.Ledger_index == b.Tx.Ledger_index && a.Meta.TransactionIndex < b.Meta.TransactionIndex {
		return true
	}
	return false
}
func (a *TxMeta) After(b *TxMeta) bool {
	if a.Tx.Ledger_index > b.Tx.Ledger_index {
		return true
	} else if a.Tx.Ledger_index == b.Tx.Ledger_index && a.Meta.TransactionIndex > b.Meta.TransactionIndex {
		return true
	}
	return false
}
