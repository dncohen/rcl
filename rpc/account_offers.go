package rpc

import "github.com/rubblelabs/ripple/data"

// {
//     "account": "rpP2JgiMyTF5jR5hLG3xHCPi1knBb1v9cM",
//     "ledger_index": "current"
// }

type AccountOffersParams struct {
	Account string `json:"account"`
}

// {
//   "id": 9,
//   "status": "success",
//   "type": "response",
//   "result": {
//     "account": "rpP2JgiMyTF5jR5hLG3xHCPi1knBb1v9cM",
//     "ledger_current_index": 18539550,
//     "offers": [
//       {
//         "flags": 0,
//         "quality": "0.00000000574666765650638",
//         "seq": 6577664,
//         "taker_gets": "33687728098",
//         "taker_pays": {
//           "currency": "EUR",
//           "issuer": "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
//           "value": "193.5921774819578"
//         }
//       },
//       {
//         "flags": 0,
//         "quality": "7989247009094510e-27",
//         "seq": 6572128,
//         "taker_gets": "2361918758",
//         "taker_pays": {
//           "currency": "XAU",
//           "issuer": "rrh7rf1gV2pXAoqA8oYbpHd8TKv5ZQeo67",
//           "value": "0.01886995237307572"
//         }
//       },
//       ... trimmed for length ...
//     ],
//     "validated": false
//   }
// }

type AccountOffersResult struct {
	Result
	Account              string              `json:"account"`
	Ledger_current_index uint32              `json:"ledger_current_index"`
	Offers               []data.AccountOffer `json:"offers"`
}
