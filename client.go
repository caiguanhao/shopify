package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
)

type (
	Client struct {
		Shop       string
		httpClient *http.Client
	}

	GqlData struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables,omitempty"`

		client *Client
		ctx    context.Context
	}

	Errors []Error

	Error struct {
		Message   string `json:"message"`
		Locations []struct {
			Line   int
			Column int
		} `json:"locations"`
		Path []interface{} `json:"path"`
	}

	UserErrors []UserError

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
func (client *Client) New(gql string, variables ...interface{}) *GqlData {
	d := &GqlData{Query: gql, client: client}
	if len(variables) > 0 {
		var key *string = nil
		for _, item := range variables {
			if key == nil {
				if s, ok := item.(string); ok {
					key = &s
				}
			} else {
				if d.Variables == nil {
					d.Variables = map[string]interface{}{}
				}
				d.Variables[*key] = item
				key = nil
			}
		}
	}
	return d
}

// Set context.
func (d *GqlData) WithContext(ctx context.Context) *GqlData {
	d.ctx = ctx
	return d
}

// MustDo is like Do but panics if operation fails.
func (d *GqlData) MustDo(dest ...interface{}) {
	if err := d.Do(dest...); err != nil {
		panic(err)
	}
}

// Execute the GraphQL query or mutation and unmarshal JSON response into the
// optional dest. Specify JSON path after each dest to efficiently get required
// info from deep nested structs.
func (d *GqlData) Do(dest ...interface{}) error {
	url := fmt.Sprintf("https://%s.myshopify.com/admin/api/2021-10/graphql.json", d.client.Shop)
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	var ctx context.Context
	if d.ctx == nil {
		ctx = context.Background()
	} else {
		ctx = d.ctx
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := d.client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("response status is not ok: %d", res.StatusCode)
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
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
	return json.Unmarshal(*resp.Data, dest[0])
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
		if v.Kind() == reflect.Slice {
			v.Set(reflect.Append(v, items[n]))
		} else {
			v.Set(items[n])
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
