package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strings"
)

var (
	ErrUnauthorized = errors.New("401 Unauthorized: incorrect authentication credential")
)

type (
	Client struct {
		Debug      bool   // print request and response body if true
		Shop       string // shop name
		httpClient *http.Client
	}

	Request struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables,omitempty"`

		client *Client
		ctx    context.Context
	}

	Errors []Error

	// Error response
	Error struct {
		Message   string `json:"message"`
		Locations []struct {
			Line   int
			Column int
		} `json:"locations"`
		Path []interface{} `json:"path"`
	}

	UserErrors []UserError

	// Error of mutation input
	UserError struct {
		Field   []string `json:"field"`
		Message string   `json:"message"`
	}

	gqlResponse struct {
		Data   *json.RawMessage `json:"data"`
		Errors Errors           `json:"errors"`
	}

	gqlResponseUserErrors struct {
		Data map[string]map[string]*UserErrors `json:"data"`
	}
)

// Create a new client with shop name and http client.
func NewClient(shop string, httpClient *http.Client) *Client {
	if t, ok := httpClient.Transport.(*Oauth2Transport); ok {
		httpClient.Transport = &transport{t}
	}
	return &Client{
		Shop:       shop,
		httpClient: httpClient,
	}
}

// Prepare a new GraphQL query or mutation. Variables must be provided in
// key-value pair order.
func (client *Client) New(gql string, variables ...interface{}) *Request {
	req := &Request{Query: gql, client: client}
	if len(variables) > 0 {
		var key *string = nil
		for _, item := range variables {
			if key == nil {
				if s, ok := item.(string); ok {
					key = &s
				}
			} else {
				if req.Variables == nil {
					req.Variables = map[string]interface{}{}
				}
				req.Variables[*key] = item
				key = nil
			}
		}
	}
	return req
}

// Set context.
func (req *Request) WithContext(ctx context.Context) *Request {
	req.ctx = ctx
	return req
}

// MustDo is like Do but panics if operation fails.
func (req *Request) MustDo(dest ...interface{}) {
	if err := req.Do(dest...); err != nil {
		panic(err)
	}
}

// Execute the GraphQL query or mutation and unmarshal JSON response into the
// optional dest. Specify JSON path after each dest to efficiently get required
// info from deep nested structs.
func (req *Request) Do(dest ...interface{}) error {
	url := fmt.Sprintf("https://%s.myshopify.com/admin/api/2021-10/graphql.json", req.client.Shop)
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var ctx context.Context
	if req.ctx == nil {
		ctx = context.Background()
	} else {
		ctx = req.ctx
	}
	if req.client.Debug {
		log.Println("[GQLReqURL] ", url)
		log.Println("[GQLReqBody]", string(data))
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
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
		log.Println("[GQLResBody]", string(b))
	}
	if res.StatusCode == 401 {
		return ErrUnauthorized
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("response status is not ok: %d", res.StatusCode)
	}

	var resp gqlResponse
	err = json.Unmarshal(b, &resp)
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return resp.Errors
	}

	var respWithUserErrors gqlResponseUserErrors
	json.Unmarshal(b, &respWithUserErrors)
	for key := range respWithUserErrors.Data {
		userErrors := respWithUserErrors.Data[key]["userErrors"]
		if userErrors == nil || len(*userErrors) == 0 {
			continue
		}
		return userErrors
	}

	if len(dest) == 0 {
		return nil
	}
	if resp.Data == nil {
		return errors.New("empty data")
	}
	if len(dest) > 1 {
		for n := 0; n < len(dest)/2; n++ {
			arrange(*resp.Data, dest[2*n], dest[2*n+1].(string))
		}
		return nil
	}
	if x, ok := dest[0].(*[]byte); ok {
		*x = *resp.Data
		return nil
	}
	return json.Unmarshal(*resp.Data, dest[0])
}

// Turn any slice into slice of interface.
func Slice(slice interface{}) (out []interface{}) {
	rv := reflect.ValueOf(slice)
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			out = append(out, rv.Index(i).Interface())
		}
	}
	return
}

// Remove any zero value in a slice.
func RemoveZeroValues(slice interface{}) (out []interface{}) {
	rv := reflect.ValueOf(slice)
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			if rv.Index(i).IsZero() {
				continue
			}
			out = append(out, rv.Index(i).Interface())
		}
	}
	return
}

func arrange(data []byte, target interface{}, key string) {
	keys := strings.Split(key, ".")
	baseType := reflect.TypeOf(target).Elem()
	if baseType.Kind() == reflect.Slice {
		baseType = baseType.Elem()
	}
	typ := baseType
	for i := len(keys) - 1; i > -1; i-- {
		key := keys[i]
		if key == "*" {
			typ = reflect.SliceOf(typ)
		} else if key != "" {
			typ = reflect.MapOf(reflect.TypeOf(key), typ)
		}
	}
	d := reflect.New(typ)
	json.Unmarshal(data, d.Interface())
	items := collect(d.Elem(), keys)
	v := reflect.Indirect(reflect.ValueOf(target))
	for n := range items {
		item := items[n]
		if !item.IsValid() {
			item = reflect.New(baseType).Elem()
		}
		if v.Kind() == reflect.Slice {
			v.Set(reflect.Append(v, item))
		} else {
			v.Set(item)
		}
	}
}

func collect(x reflect.Value, keys []string) (out []reflect.Value) {
	for i, key := range keys {
		if key == "*" {
			k := keys[i+1:]
			for i := 0; i < x.Len(); i++ {
				out = append(out, collect(x.Index(i), k)...)
			}
			return
		} else if key != "" {
			x = x.MapIndex(reflect.ValueOf(key))
		}
	}
	out = append(out, x)
	return
}

func (errs Errors) Error() string {
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Message)
	}
	return strings.Join(msgs, ", ")
}

func (errs UserErrors) Error() string {
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Message)
	}
	return strings.Join(msgs, ", ")
}
