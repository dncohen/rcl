package rpc

import (
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"
)

type SubmitParams struct {
	Tx_blob   string `json:"tx_blob"`
	Fail_hard bool   `json:"fail_hard"`
}

// {"engine_result":"tesSUCCESS","engine_result_code":0,"engine_result_message":"The transaction was applied. Only final in a validated ledger.","status":"success","tx_blob":"1200032400000001202100000002684000000000000014732103364EA1B298A4BE65CE1C9FD6EA31B273BFE8F07E953378F20D65A0C166CE8AEA74473045022100CFB13416D99E66148665F008C7A44B6A4D9C8D15DCCB8368D6797279DFF3E802022034B86EBA695EB022A9863F4D41430C0CB32D5D714A4B60D2D2717B2CF26F28C48114E55D92BCED7D00CA95A59EB3F10AD661D668D01A","tx_json":{"Account":"rMumqWWTVhGAo1Zud8dFDzS9N5RNEDK1JS","Fee":"20","Sequence":1,"SetFlag":2,"SigningPubKey":"03364EA1B298A4BE65CE1C9FD6EA31B273BFE8F07E953378F20D65A0C166CE8AEA","TransactionType":"AccountSet","TxnSignature":"3045022100CFB13416D99E66148665F008C7A44B6A4D9C8D15DCCB8368D6797279DFF3E802022034B86EBA695EB022A9863F4D41430C0CB32D5D714A4B60D2D2717B2CF26F28C4","hash":"EAAE1D146AE3D5F6A0592CB379129C5D0410FA9E4C4AFCFF543EA7E67AF2AA43"}}

type ResultSubmit struct {
	Result
	Engine_result         string   `json:"engine_result"`
	Engine_result_message string   `json:"engine_result_message"`
	Tx_json               TxResult `json:"tx_json"` // Needed for hash and possibly more
	Tx_blob               string   `json:"tx_blob"`
}

func (result ResultSubmit) String() string {
	return fmt.Sprintf("%s (%s): %s", result.Engine_result, result.Engine_result_message, result.Tx_json.String())
}

// Submit a signed transaction blob.
// TODO: should this be moved to rpc package?
func (client Client) Submit(blob string) (ResultSubmit, error) {
	response := ResultSubmit{}

	_, err := client.Do("submit", SubmitParams{
		Tx_blob:   blob,
		Fail_hard: true,
	}, &response)
	if err != nil {
		return response, err
	}

	if response.Engine_result == "tesSUCCESS" {
		// Tentative success.
	} else if strings.HasPrefix(response.Engine_result, "tec") {
		// Tentative failure that consumes a fee and does get propagated to network.
		log.Printf("Tentative %s", response)
	} else {
		log.Printf("Submit failed %s", response)
		// Failure that does not get propagated when fail_hard is true.
		err = errors.Errorf("Transaction %s failed with %s %s", response.Tx_json.Hash, response.Engine_result, response.Engine_result_message)
	}

	return response, err
}
