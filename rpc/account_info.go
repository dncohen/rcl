package rpc

type AccountInfoParams struct {
	Account      string `json:"account"`
	Ledger_index *int32 `json:"ledger_index,omitempty"` // TODO: support "current", "closed", "validated"
}

// account_info
// {"account_data":{"Account":"rs2GgdxJx34DwwAUsz1wse3yUCnggQpCCg","Balance":"10000000000","Flags":0,"LedgerEntryType":"AccountRoot","OwnerCount":0,"PreviousTxnID":"F295A38531D6808917F6B42A5E583F89D0613C0153096F497648C771EADE183A","PreviousTxnLgrSeq":1918860,"Sequence":1,"index":"3066338D048B57636FA27F4027619FD8910AF9C1E2F2148AECA288B1B85D8E9F"},"ledger_current_index":1974161,"status":"success","validated":false}
type AccountInfoResult struct {
	Result
	Account_data AccountInfo
}
type AccountInfo struct {
	Account              string
	Balance              uint64 `json:",string"`
	Flags                uint32
	LedgerEntryType      string
	OwnerCount           uint32
	PreviousTxnId        string
	PrevioutTxnLedgerSeq uint32 // correct type?
	Sequence             uint32
	Index                string
}
