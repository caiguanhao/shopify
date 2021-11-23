package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type (
	KV = map[string]interface{}

	RestRequest struct {
		method string
		route  string
		client *Client
		ctx    context.Context
		query  interface{}
		body   interface{}
	}

	restErrorString struct {
		Errors string `json:"errors"`
	}

	restErrorStrings struct {
		Errors []string `json:"errors"`
	}

	restErrorKeyValue struct {
		Errors map[string]string `json:"errors"`
	}
)

// Prepare a new restful request given method and route name. First params is
// request query and second params is request body.
func (client *Client) NewRest(method, route string, params ...interface{}) *RestRequest {
	req := &RestRequest{
		method: method,
		route:  route,
		client: client,
	}
	if len(params) > 0 {
		req.query = params[0]
	}
	if len(params) > 1 {
		req.body = params[1]
	}
	return req
}

// Set context.
func (req *RestRequest) WithContext(ctx context.Context) *RestRequest {
	req.ctx = ctx
	return req
}

// MustDo is like Do but panics if operation fails.
func (req *RestRequest) MustDo(dest ...interface{}) {
	if err := req.Do(dest...); err != nil {
		panic(err)
	}
}

// Execute the restful request and unmarshal JSON response into the optional
// dest. Specify JSON path after each dest to efficiently get required info
// from deep nested structs.
func (req *RestRequest) Do(dest ...interface{}) error {
	reqUrl := fmt.Sprintf("https://%s.myshopify.com/admin/api/2021-10/%s.json", req.client.Shop, req.route)
	var values url.Values
	switch query := req.query.(type) {
	case map[string]string:
		values = url.Values{}
		for k, v := range query {
			values.Set(k, v)
		}
	case KV:
		values = url.Values{}
		for k, v := range query {
			values.Set(k, fmt.Sprint(v))
		}
	case url.Values:
		values = query
	}
	if qs := values.Encode(); qs != "" {
		reqUrl += "?" + qs
	}
	if req.client.Debug {
		log.Println("[RestReqURL] ", reqUrl)
	}
	var body io.Reader
	if req.body != nil {
		data, err := json.Marshal(req.body)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(data)
		if req.client.Debug {
			log.Println("[RestReqBody]", string(data))
		}
	}
	var ctx context.Context
	if req.ctx == nil {
		ctx = context.Background()
	} else {
		ctx = req.ctx
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.method, reqUrl, body)
	if err != nil {
		return err
	}
	httpReq.Header.Add("Content-Type", "application/json")
	res, err := req.client.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if req.client.Debug {
		log.Println("[RestResBody]", string(b))
	}

	var errStr restErrorString
	json.Unmarshal(b, &errStr)
	if errStr.Errors != "" {
		return errStr
	}

	var errStrs restErrorStrings
	json.Unmarshal(b, &errStrs)
	if len(errStrs.Errors) > 0 {
		return errStrs
	}

	var errKV restErrorKeyValue
	json.Unmarshal(b, &errKV)
	if len(errKV.Errors) > 0 {
		return errKV
	}

	if res.StatusCode == 401 {
		return ErrUnauthorized
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("response status is not ok: %d", res.StatusCode)
	}
	if len(dest) == 0 {
		return nil
	}
	if len(dest) > 1 {
		for n := 0; n < len(dest)/2; n++ {
			arrange(b, dest[2*n], dest[2*n+1].(string))
		}
		return nil
	}
	return json.Unmarshal(b, dest[0])
}

func (e restErrorString) Error() string {
	return e.Errors
}

func (e restErrorStrings) Error() string {
	return strings.Join(e.Errors, ", ")
}

func (e restErrorKeyValue) Error() string {
	var msgs []string
	for k, v := range e.Errors {
		msgs = append(msgs, k+": "+v)
	}
	return strings.Join(msgs, ", ")
}
