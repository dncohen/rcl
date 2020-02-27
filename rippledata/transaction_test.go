package rippledata

import (
	"os"
	"path/filepath"
	"testing"
)

// unmarshal SignerListSet
func TestDD6B591818063835ED0AF903178E4859E0E6DD049A665294A0E3131C7F8094EE(t *testing.T) {
	txHash := "DD6B591818063835ED0AF903178E4859E0E6DD049A665294A0E3131C7F8094EE"
	f, err := os.Open("testdata/tx" + txHash + ".json")
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()

	txr := &GetTransactionResponse{}
	err = unmarshal(txr, f)
	if err != nil {
		t.Error(err)
		return
	}

	//pretty.Log("unmarshalled transaction:", txr.Transaction)
	// t.Log(string(txr.getRaw())) // verbose

	if txr.Transaction.Hash.String() != txHash {
		t.Errorf("wanted %s, got %q", txHash, txr.Transaction.Hash)
	}

}

func TestUnmarshalTransaction(t *testing.T) {
	file, err := filepath.Glob("testdata/tx*.json")
	if err != nil {
		t.Error(err)
		return
	}
	for _, fname := range file {
		f, err := os.Open(fname)
		if err != nil {
			t.Error(err)
			continue
		}
		defer f.Close()

		txr := &GetTransactionResponse{}
		err = unmarshal(txr, f)
		if err != nil {
			t.Error(err)
			continue
		}

		//pretty.Log("unmarshalled transaction:", txr.Transaction) // verbose

		if txr.Transaction.Hash != *txr.Transaction.Tx.GetHash() {
			t.Errorf("unmarshalled transaction: wanted %q, got %q", txr.Transaction.Hash, txr.Transaction.Tx.GetHash())
		}
	}
}
