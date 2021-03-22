package rippledata

import (
	"encoding/json"
	"fmt"
	"io"
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

var errGetFail = errors.New("GET failed")
var errStupid = errors.New("rippledata API fail")

// Get method submits a query to the Ripple Data API and returns response.  If a request fails, Get retries, possibly several times.
func (this Client) Get(response DataResponse, endpoint *url.URL, values *url.Values) error {
	count := 0
	// retry several times if necessary, because data api is intermittently unavailable
	// may be caused by rate limiter
	for {
		count++
		err := this.get(response, endpoint, values)
		if err != nil && (errors.Is(err, errStupid)) {
			if count > 10 {
				return errors.Wrapf(err, "rippledata GET failed (%d attempts)", count)
			}
			log.Printf("failed attempt %d to GET %s: %s\n", count, endpoint, err) // verbose
			<-time.After(time.Duration(count) * 30 * time.Second)                 // wait between attempts
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

	err = unmarshal(response, res.Body)
	if err != nil {
		return fmt.Errorf("GET %s: %w", endpoint, err)
	}
	return nil
}

func unmarshal(response DataResponse, r io.Reader) error {

	type triager interface {
		triage(json.RawMessage) json.RawMessage
	}
	type postoper interface {
		postop()
	}

	// keep the raw, for decoding that may be tricky depending on the endpoint
	var raw json.RawMessage
	err := json.NewDecoder(r).Decode(&raw)
	if err != nil {
		return err
	}

	// apparently ripple put a rate limiter in front of the API, and it
	// returns a JSON dialect with no relation to the rest of the API.

	var stupid = struct {
		Error string `json:"error"`
	}{}
	err = json.Unmarshal(raw, &stupid)
	if err == nil && stupid.Error != "" {
		return fmt.Errorf("%w: %s", errStupid, stupid.Error)
	}

	triage, ok := response.(triager)
	if ok {
		raw = triage.triage(raw) // kludge to align what Data API returns to be more like what rippled API returns
	}

	// Now get the result from the raw
	response.setRaw(raw)

	err = json.Unmarshal(raw, response)
	if err == nil && response.GetResult() != "success" {
		log.Println(string(raw))
		return fmt.Errorf("result %q, response %q: %w", response.GetResult(), response.GetMessage(), errGetFail)
	}

	postop, ok := response.(postoper)
	if ok {
		postop.postop()
	}

	return err
}

type DataResponse interface {
	getRaw() json.RawMessage
	setRaw(json.RawMessage)
	GetResult() string
	GetMessage() string
}

// Response is a generic response to a Data API request.
type Response struct {
	Result  string `json:"result"`            // "success" expected
	Message string `json:"message,omitempty"` // present when  "result": "error"

	// When unmarshalling, save the raw bytes for further unmarshalling
	// into type-specific structs.
	raw json.RawMessage
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
