package util

import (
	"fmt"
	"log"

	"github.com/rubblelabs/ripple/data"
)

var (
	reasonableFee                 *data.Value
	reasonableLedgerSequenceDelta = uint32(10)
)

func init() {
	var err error
	reasonableFee, err = data.NewNativeValue(20)
	if err != nil {
		log.Panicf("Failed data.NewNativeValue(): %s", err)
	}
}

func FormatTransactionWithMetaDataHeader() string {
	return fmt.Sprintf("%s\t %s - %s\t %s - %s\t %s\t %s\t %s",
		"Transaction",
		"Ledger", "index", // Ledger and order within ledger
		"Account", "sequence", // Account and sequence number
		"Type",
		"Result", "Description")
}
func FormatTransactionWithMetaDataRow(tx *data.TransactionWithMetaData) string {
	return fmt.Sprintf("%s\t %d - %d\t %s - %d\t %s\t %s\t %s",
		tx.GetHash(),
		tx.LedgerSequence, tx.MetaData.TransactionIndex, // Ledger and order within ledger
		tx.GetBase().Account, tx.GetBase().Sequence, // Account and sequence number
		tx.GetType(),
		tx.MetaData.TransactionResult,
		formatTransactionDescription(tx))
}

func formatTransactionDescription(txm *data.TransactionWithMetaData) string {
	if txm.MetaData.TransactionResult.Success() {

		switch tx := txm.Transaction.(type) {
		case *data.Payment:
			return fmt.Sprintf("Delivered %s to %s",
				txm.MetaData.DeliveredAmount, tx.Destination)
		default:
			return ""
		}
	} else {
		return ""
	}
}

// Analyze transaction for possible flaws.  Format as human readable strings.
func LintTransaction(txm *data.TransactionWithMetaData) []string {
	lint := make([]string, 0)

	if txm.GetBase().Flags == nil || (data.TxCanonicalSignature&*txm.GetBase().Flags) == 0 {
		lint = append(lint, fmt.Sprintf("tfFullyCanonicalSig transaction flag not set"))
	}

	fee := txm.GetBase().Fee
	if reasonableFee.Less(fee) {
		lint = append(lint, fmt.Sprintf("Paid fee %s XRP", fee))
	}
	if txm.GetBase().LastLedgerSequence == nil {
		lint = append(lint, "LastLedgerSequence omitted")
	} else {
		delta := *txm.GetBase().LastLedgerSequence - txm.Ledger()
		if delta > reasonableLedgerSequenceDelta {
			lint = append(lint, fmt.Sprintf("LastLedgerSequence exceeds ledger sequence by %d", delta))
		}
	}

	// Transaction type specific checks...
	switch tx := txm.Transaction.(type) {
	case *data.Payment:
		if txm.MetaData.DeliveredAmount.Value.Less(*tx.Amount.Value) {
			lint = append(lint, fmt.Sprintf("Partial payment delivered %s of %s",
				txm.MetaData.DeliveredAmount, tx.Amount))
		}
	}

	return lint
}
