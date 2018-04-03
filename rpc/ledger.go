package rpc

// https://ripple.com/build/rippled-apis/#ledger

type LedgerParams struct {
	Ledger_index *int32 `json:"ledger_index,omitempty"` // TODO: support "current", "closed", "validated"
	Accounts     bool   `json:"accounts"`
	Full         bool   `json:"full"`
	Transactions bool   `json:"transactions"`
	Expand       bool   `json:"expand"`
	// TODO other options https://ripple.com/build/rippled-apis/#ledger
}

type LedgerResult struct {
	Result
	Ledger LedgerInfo
}

type LedgerInfo struct {
	Ledger_index string // yes, string
	Ledger_hash  string
	Account_hash string
	// TODO more fields https://ripple.com/build/rippled-apis/#ledger

	// TODO Transactions can be just hashes, if expand == false
	Transactions []TxResult // Is this right?
}
