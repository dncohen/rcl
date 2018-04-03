// Helpers to make JSON-RPC calls to rippled.
package rpc

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/y0ssar1an/q"
)

const (
	DropsPerXRP = 1000000
)

type Request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// JSON-RPC response {"result":{..., "status": "..."}}
type Response struct {
	Result    json.RawMessage `json:"result"`
	Validated *bool           `json:"validated"` // Appears in response to "tx"
	code      int
	status    string
}

func (r Response) IsValidated() bool {
	return r.Validated != nil && *r.Validated
}

// Examples of potential errors:
// {"result":{"error":"unknownCmd","error_code":31,"error_message":"Unknown method.","request":{"command":"server_infoX"},"status":"error"}}
// {"error":"invalidTransaction","error_exception":"Unknown field","request":{"command":"submit","tx_blob":"12000024000000012021000000026880000000000000007321026C80D7F11B33BE4E2794E489295346ACC31F0719DB2FA6C378E6E7BF325873A074473045022100F9123F923267F3E91539397C8353DD1BE55C531DD9F9D18B662A7B3035E0BA5502200F2F2B18DD3277B7575E2792A0FD9A08FC2C3888F0ACE14598AC5F67F05F208C81141C3B11F542BBA14819B579D59FBA1EB17457B2BB"},"status":"error"}
type Result struct {
	// Always present:
	Status string

	// Present only when error:
	Error         string `json:"error,omitempty"`
	Error_code    int
	Error_message string
	Request       json.RawMessage

	// Other errors
	Error_exception string `json:"error_exception"`

	// Present sometimes:
	Ledger_current_index int // type ???
	Validated            bool
}

type Client struct {
	url      string
	insecure bool
	client   http.Client
}

func (c Client) String() string {
	return fmt.Sprintf("JSON-RPC via %s", c.url)
}

func NewClient(url string, insecure bool) (Client, error) {
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	httpClient := http.Client{Transport: &tr}

	client := Client{
		url:      url,
		insecure: insecure,
		client:   httpClient,
	}

	return client, nil
}

func (client Client) Close() {
	// Is there anything needs closing?
}

// Helpers for rippled JSON-RPC calls.

// TODO: re-order params to Do() and allow any number of ...params
// Do() is being deprecated, use Request() below.
func (client Client) Do(method string, params interface{}, target interface{}) (int, error) {
	// target should be an Result
	//_ = target.(*Result) // <-- does not work as hoped with golang embedding

	var jsonBytes []byte
	var err error

	// For historical reasons, params can be nil, or a string.  Both deprecated.
	if params == nil {
		params = "[{}]"
	}
	paramStr, ok := params.(string)
	if ok {
		j := "{\"method\":\"" + method + "\", \"params\":" + paramStr + "}"
		jsonBytes = []byte(j)
	} else {
		// Wrap a single param into an array.
		// TODO: add support for array of params, to batch process multiple requests.
		rpcParams := make([]interface{}, 1, 1)
		rpcParams[0] = params
		rpcReq := Request{Method: method, Params: rpcParams}
		//q.Q(rpcReq)
		jsonBytes, err = json.Marshal(rpcReq)
		if err != nil {
			return 0, err
		}
		q.Q(string(jsonBytes)) // debug
	}

	//req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	//if err != nil {
	//	return 0, errors.Wrapf(err, "POST %s", url)
	//}

	//req.Header.Set("Content-Type", "application/json")
	//client := &http.Client{}
	//resp, err := client.Do(req)

	resp, err := client.client.Post(client.url, "application/json", bytes.NewBuffer(jsonBytes))

	if err != nil {
		q.Q(method, params)
		return 0, errors.Wrapf(err, "POST %s", client.url)
	}
	defer resp.Body.Close()

	// Debug
	q.Q("REQUEST TO: ", client.url, method)
	q.Q("BODY: ", string(jsonBytes))
	q.Q(resp.Status) // debug

	if resp.StatusCode != 200 {
		q.Q(resp)
		err = errors.New(resp.Status)
		return resp.StatusCode, err
	}

	response, ok := target.(*Response)
	if ok {
		err = json.NewDecoder(resp.Body).Decode(target)
		if err != nil {
			q.Q(err)
			return resp.StatusCode, err
		}
	} else {
		response = &Response{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {

			if unmarshalErr, ok := err.(*json.UnmarshalTypeError); ok {
				// Output more information to make this error easier to fix.
				log.Println("Failed to unmarshal RPC response!", unmarshalErr)
				test := json.RawMessage{}
				e := json.NewDecoder(resp.Body).Decode(&test) // Can we do this again?
				if e != nil {
					q.Q(e)
				} else {
					q.Q(string(test))
				}
			}

			q.Q(err)
			return resp.StatusCode, err
		}

		err = json.Unmarshal(response.Result, target)
		if err != nil {
			if unmarshalErr, ok := err.(*json.UnmarshalTypeError); ok {
				// Output more information to make this error easier to fix.
				log.Println("Failed to unmarshal RPC response!", unmarshalErr)
				test := json.RawMessage{}
				e := json.Unmarshal(response.Result, &test)
				if e != nil {
					q.Q(e)
				} else {
					q.Q(string(test))
				}
			}

			q.Q(err)
			return resp.StatusCode, err
		}

		q.Q(target)
	}

	// Unmarshal purely as a test of the result status.  Have not found a good way to avoid this additional unmarshal.
	result := Result{}
	err = json.Unmarshal(response.Result, &result)

	// Did we get an error back?
	if result.Status != "success" { // Note err == nil even when result does not have the error fields.
		// We decoded an error.
		return resp.StatusCode, errors.Errorf("POST %s to %s returned %s: %s %s  (Request: %s)", method, client.url, result.Error, result.Error_message, result.Error_exception, string(result.Request))
	} else {
		// We did not decode an error.
		return resp.StatusCode, err
	}
}

func (res Response) StatusCode() int {
	return res.code
}

func (res Response) ResultString() string {
	return string(res.Result)
}

func (res Response) String() string {
	byte, err := json.Marshal(res)
	if err != nil {
		return fmt.Sprintf("Error %T: %q", err, err)
	}
	//return string(bytes) // Non-pretty string

	// Pretty indent
	var out bytes.Buffer
	err = json.Indent(&out, byte, "", "\t")
	if err != nil {
		return string(byte)
	}
	return out.String()
}

func (res Response) UnmarshalResult(v interface{}) error {
	return json.Unmarshal(res.Result, v)
}

func (client Client) Request(method string, params ...interface{}) (*Response, error) {
	req := Request{Method: method, Params: params}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	res, err := client.client.Post(client.url, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, errors.Wrapf(err, "ripple rpc %s %s", method, client.url)
	}
	defer res.Body.Close()

	response := Response{
		code: res.StatusCode,
	}

	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return &response, err
	}

	response.status = jsoniter.Get(response.Result, "status").ToString()

	// Did we get an error back?
	if response.status != "success" {
		result := Result{}
		err = response.UnmarshalResult(&result)
		if err != nil {
			return &response, errors.Wrapf(err, "Failed to parse error detail")
		}
		return &response, errors.Errorf("POST %s to %s returned %s: %s %s  (Request: %s)", method, client.url, result.Error, result.Error_message, result.Error_exception, string(result.Request))
	}

	return &response, err
}
