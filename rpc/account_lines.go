package rpc

import "github.com/rubblelabs/ripple/data"

//         {
//            "account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//            "ledger": "current"
//        }

type AccountLinesParams struct {
	Account string `json:"account"`
}

// account_lines
// {
//     "result": {
//         "account": "r9cZA1mLK5R5Am25ArfXFmqgNwjZgnfk59",
//         "lines": [
//             {
//                 "account": "r3vi7mWxru9rJCxETCyA1CHvzL96eZWx5z",
//                 "balance": "0",
//                 "currency": "ASP",
//                 "limit": "0",
//                 "limit_peer": "10",
//                 "quality_in": 0,
//                 "quality_out": 0
//             },
//             {
//                 "account": "r3vi7mWxru9rJCxETCyA1CHvzL96eZWx5z",
//                 "balance": "0",
//                 "currency": "XAU",
//                 "limit": "0",
//                 "limit_peer": "0",
//                 "no_ripple": true,
//                 "no_ripple_peer": true,
//                 "quality_in": 0,
//                 "quality_out": 0
//             },
//             {
//                 "account": "rs9M85karFkCRjvc6KMWn8Coigm9cbcgcx",
//                 "balance": "0",
//                 "currency": "015841551A748AD2C1F76FF6ECB0CCCD00000000",
//                 "limit": "10.01037626125837",
//                 "limit_peer": "0",
//                 "no_ripple": true,
//                 "quality_in": 0,
//                 "quality_out": 0
//             }
//         ],
//         "status": "success"
//     }
// }

type AccountLinesResult struct {
	Result
	Account string        `json:"account"`
	Lines   []AccountLine `json:"lines"`
}

type AccountLine struct {
	Account        string     `json:"account"`
	Balance        data.Value `json:"balance"`
	Currency       string     `json:"currency"`
	Limit          data.Value `json:"limit"`
	Limit_peer     data.Value `json:"limit_peer"`
	No_ripple      bool       `json:"no_ripple"`
	No_ripple_peer bool       `json:"no_ripple_peer"`
	Quality_in     uint32     `json:"quality_in"`
	Quality_out    uint32     `json:"quality_out"`
	Freeze         bool       `json:"freeze"`
	Freeze_peer    bool       `json:"freeze_peer"`
}
