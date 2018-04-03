package rpc

import "fmt"

// "{\"info\":{\"build_version\":\"0.70.0-b8\",\"complete_ledgers\":\"1955521-1967811\",\"hostid\":\"HI\",\"io_latency_ms\":1,\"last_close\":{\"converge_time_s\":1.999,\"proposers\":4},\"load_factor\":1,\"peers\":5,\"pubkey_node\":\"n9KMmZw85d5erkaTv62Vz6SbDJSyeihAEB3jwnb3Bqnr2AydRVep\",\"server_state\":\"proposing\",\"state_accounting\":{\"connected\":{\"duration_us\":\"4999978\",\"transitions\":1},\"disconnected\":{\"duration_us\":\"1262060\",\"transitions\":1},\"full\":{\"duration_us\":\"410736965528\",\"transitions\":1},\"syncing\":{\"duration_us\":\"5002153\",\"transitions\":1},\"tracking\":{\"duration_us\":\"2\",\"transitions\":1}},\"uptime\":410748,\"validated_ledger\":{\"base_fee_xrp\":1e-05,\"hash\":\"77147A57D2351EB97F6F6C709B94364E6B9C47525D0D02E7958575F4AB525BF4\",\"reserve_base_xrp\":20,\"reserve_inc_xrp\":5,\"seq\":1967811},\"validation_quorum\":4},\"status\":\"success\"}",
type ServerInfoResult struct {
	Result
	Info ServerInfo
}
type ServerInfo struct {
	Result
	Build_version    string
	Complete_ledgers string
	Hostid           string
	Load_factor      float64 // Not integer!
	Peers            int
	Pubkey_node      string
	Server_state     string
	Uptime           int
	Validated_ledger *ValidatedLedger
}
type ValidatedLedger struct {
	Base_fee_xrp     float64 // ???
	Hash             string
	Reserve_base_xrp int
	Reserve_inc_xrp  int
	Seq              int
}

// Estimates the current transaction fee, in drops.  Computed as
// described in https://ripple.com/build/transaction-cost/#server-info
func (server *ServerInfo) Fee() int {
	baseFeeDrops := server.Validated_ledger.Base_fee_xrp * DropsPerXRP
	return int(baseFeeDrops * server.Load_factor)
}

func (info *ServerInfoResult) String() string {
	return fmt.Sprintf("%s ledgers: %s peers: %d, load_factor: %f, uptime: %d", info.Info.Hostid, info.Info.Complete_ledgers, info.Info.Peers, info.Info.Load_factor, info.Info.Uptime)
}
