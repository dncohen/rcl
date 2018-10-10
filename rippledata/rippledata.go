package rippledata

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"
)

// https://ripple.com/build/data-api-v2

type Client struct {
	base   *url.URL
	client http.Client
}

func NewClient(base string) (Client, error) {
	var err error

	tr := http.Transport{
	//TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	httpClient := http.Client{Transport: &tr}

	client := Client{
		client: httpClient,
	}

	client.base, err = url.Parse(base)

	// TODO: perhaps test that url is reachable.

	return client, err

}

func (client Client) Close() {
	// Is there anything needs closing?
}

// Produces a url for an endpoint.  panics on error in order to allow inline calling.
func (this Client) Endpoint(segment ...string) *url.URL {
	endpoint, err := this.base.Parse(path.Join(segment...))
	if err != nil {
		log.Panicln(err, "rippledata: Cannot produce endpoint")
	}
	return endpoint
}

var getError = errors.New("GET failed")

// Retry requests, because ripple data has trouble, pretty often.
func (this Client) Get(response DataResponse, endpoint *url.URL, values *url.Values) error {
	count := 0
	for {
		count++
		err := this.get(response, endpoint, values)
		if err != nil && errors.Cause(err) == getError {
			if count > 10 {
				return errors.Wrapf(err, "rippledata GET failed (%d attempts)", count)
			}
			<-time.After(time.Duration(count) * time.Second) // wait between attempts
		} else {
			return err
		}
	}
}

func (this Client) get(response DataResponse, endpoint *url.URL, values *url.Values) error {
	if values != nil {
		endpoint.RawQuery = values.Encode()
	}

	res, err := this.client.Get(endpoint.String())
	if err != nil {
		return errors.Wrapf(err, "GET %s", endpoint)
	}
	defer res.Body.Close()

	//response, err := decodeResponse(res)

	var raw json.RawMessage
	err = json.NewDecoder(res.Body).Decode(&raw)
	if err != nil {
		err = errors.Wrapf(err, "GET %s could not decode response", endpoint)
		//q.Q(err, string(raw)) // debug
	}

	// Now get the result from the raw
	response.setRaw(raw)

	err = json.Unmarshal(raw, response)
	if err == nil && response.GetResult() != "success" {
		err = errors.Wrapf(getError, "GET %s returned %s: %s", endpoint.String(), response.GetResult(), response.GetMessage())
		//q.Q(err, string(raw)) // debug
	}

	return err
}

type Response struct {
	Result  string `json:"result"`            // "success" expected
	Message string `json:"message,omitempty"` // present when  "result": "error"

	// When unmarshalling, save the raw bytes for further unmarshalling
	// into type-specific structs.
	raw json.RawMessage
}

type DataResponse interface {
	getRaw() json.RawMessage
	setRaw(json.RawMessage)
	GetResult() string
	GetMessage() string
}

func (this *Response) getRaw() json.RawMessage {
	return this.raw
}
func (this *Response) setRaw(raw json.RawMessage) {
	this.raw = raw
}
func (this *Response) GetResult() string {
	return this.Result
}
func (this *Response) GetMessage() string {
	return this.Message
}

func decodeResponse(res *http.Response) (*Response, error) {
	//response := Response{}
	return nil, errors.New("decodeResponse deprecated / not implemented")
}

/*
func (client Client) Post(method string, params ...interface{}) (*Response, error) {
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
*/
