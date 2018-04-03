package util

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/dncohen/rcl/rpc"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
	"github.com/y0ssar1an/q"
)

const (
	updateIntervalSeconds = 20
	DropsPerXRP           = 1000000
)

var (
	feeIncrementFactor *data.Value
)

func init() {
	feeIncrementFactor, _ = data.NewNonNativeValue(125, -2) // 125 * 10^(-2) == 1.25
}

type ServerInfo struct {
	client *rpc.Client
	result *rpc.ServerInfoResult

	// Keep track of contiguous ledger history we have available.
	ledgerIndexMin *uint32
	ledgerIndexMax *uint32
}

func NewServerInfoUpdater(client rpc.Client) (*ServerInfo, error) {

	info := &ServerInfo{
		client: &client,
	}

	_, err := info.updateServerInfo()
	if err != nil {
		return nil, err
	}

	// Keep our info reasonably up-to-date
	go func() {
		disconnected := false

		for {
			select {
			case <-time.After(updateIntervalSeconds * time.Second):
			}
			_, err := info.updateServerInfo()
			if err != nil {
				log.Println("Failed to update server_info:", err)
				disconnected = true
			} else {
				if disconnected {
					// Log on reconnect.
					log.Printf("Connected to %s.  hostid=%s complete_ledgers=%s", client, info.Result().Info.Hostid, info.Result().Info.Complete_ledgers) // verbose!
					disconnected = false
				}
			}
		}
	}()

	return info, err
}

func (info *ServerInfo) updateServerInfo() (rpc.ServerInfoResult, error) {
	result := rpc.ServerInfoResult{}

	response, err := info.client.Request("server_info")
	if err != nil {
		return result, err
	}

	err = response.UnmarshalResult(&result)

	if err != nil {
		return result, err
	}

	// Determine the ledger history available.
	ledgerChunks := strings.Split(result.Info.Complete_ledgers, ",")
	ledgerBounds := strings.Split(ledgerChunks[len(ledgerChunks)-1], "-")
	// confirm we have valid range
	if len(ledgerBounds) != 2 {
		return result, errors.Errorf("Failed to parse complete_ledgers from server_info %s", string(response.Result))
	}
	min, err := strconv.ParseInt(ledgerBounds[0], 10, 32)
	if err == nil {
		max, err := strconv.ParseInt(ledgerBounds[1], 10, 32)
		if err == nil {
			// Only assign these if no error.
			ledgerIndexMin := uint32(min)
			ledgerIndexMax := uint32(max)
			// TODO sync
			info.result = &result
			info.ledgerIndexMin = &ledgerIndexMin
			info.ledgerIndexMax = &ledgerIndexMax
		}

		// log.Printf("Updated server_info: %s\n", info.result) // verbose
	}

	return result, err
}

func (info *ServerInfo) Result() *rpc.ServerInfoResult {
	return info.result
}
func (info *ServerInfo) LedgerIndexMin() uint32 {
	if info.ledgerIndexMin == nil {
		return 0
	}
	return *info.ledgerIndexMin
}
func (info *ServerInfo) LedgerIndexMax() uint32 {
	if info.ledgerIndexMax == nil {
		return 0
	}
	return *info.ledgerIndexMax
}
func (info *ServerInfo) LedgerIndexBounds() (uint32, uint32) {
	return info.LedgerIndexMin(), info.LedgerIndexMax()
}

func (server *ServerInfo) Fee() int {
	return server.result.Info.Fee()
}

// This is part of *ServerInfo because ledger history is known, checked vs LastLedgerSequence
func (server *ServerInfo) SignSubmitWait(t data.Transaction, keypair Keypair) (rpc.TxResult, error) {

	var result rpc.TxResult
	client := server.client
	txBase := t.GetBase()

	hash, blob, err := Sign(t, keypair)
	_ = hash
	if err != nil {
		return result, errors.Wrapf(err, "Failed to sign %s %s", txBase.TransactionType, t)
	}

	ledgerBeforeSubmit := server.LedgerIndexMax()

	tentative, err := client.Submit(blob)
	if err != nil {
		log.Printf("SignSubmitWait tentative failure: %s\n", err)
		// This might be tentative.  We don't want to return until LastLedgerSequence has passed.
	}

	if tentative.Engine_result != "tesSUCCESS" {
		// This is just a warning.  Final result could still be tesSUCCESS.
		log.Printf("Tentative result %s (%s) on (%s)\n", tentative.Engine_result, tentative.Engine_result_message, t)
	}

	// TODO: return here if status is guaranteed to never appear on a validated ledger.

	for result.IsValidated() == false {

		// For now, poll.  Future TODO websockets.
		select {
		case <-time.After(10 * time.Second):
		}

		response, err := client.Request("tx", rpc.TxParams{Transaction: tentative.Tx_json.Hash.String()})
		if err != nil {
			// TODO: fail here?
			log.Printf("Failed to look up transaction %s: %s\n", tentative.Tx_json.Hash, err)
		} else {
			//q.Q(string(response.Result)) // debug
			err = response.UnmarshalResult(&result)
			if err != nil {
				// TODO: fail here?
				log.Printf("Failed to parse transaction %s: %s\n", response, err)
			} else {

				// Confusingly, "validated" is at the top level when "tx" returns.
				// (As opposed to when "account_tx" returns)
				if response.IsValidated() || result.IsValidated() {

					log.Printf("Validated %s %s %s\n", result.TransactionType, result.Hash, result.Meta.TransactionResult)

				} else if t.GetBase().LastLedgerSequence != nil {
					// Take the LastLedgerSequence into account when deciding how long to wait.
					currentMin, currentMax := server.LedgerIndexBounds()
					ll := *t.GetBase().LastLedgerSequence
					if currentMin <= ledgerBeforeSubmit && currentMax >= ll {
						// The result is immutable because it does not appear in comprehensive ledger history.
						q.Q(result)
						log.Printf("LastLedgerSequence exceeded: %s\n", result.String())
						return result, nil
					}
				} else {

					// This is most likely terQUEUED.
					log.Printf("Tentative result %s for %s %s\n", result.Meta.TransactionResult, result.TransactionType, result.Hash)

				}
			}
		}
	}
	// Here, we have validated result.
	return result, nil

}

// This is part of *ServerInfo because ledger history is known, checked vs LastLedgerSequence
func (server *ServerInfo) SignSubmitRetry(t data.Transaction, keypair Keypair) (rpc.TxResult, error) {
	for {
		result, err := server.SignSubmitWait(t, keypair)
		if err != nil {
			log.Printf("SignSubmitWait failed: %s\n", err)
			// We don't necessarily want to give up here.  I.e. a tefMAX_LEDGER should be retried.
			//return result, err
		}

		if result.IsValidated() {
			return result, err
		} else {
			newFee, err := t.GetBase().Fee.Multiply(*feeIncrementFactor)
			if err == nil {
				t.GetBase().Fee = *newFee
			}
			newLL := uint32(server.LedgerIndexMax() + 3)
			t.GetBase().LastLedgerSequence = &newLL
			// Retry with increased Fee and LastLedgerSequence
		}
	}
}
