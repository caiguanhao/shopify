# shopify

Shopify GraphQL and Restful Admin API client

[Docs of this package](https://pkg.go.dev/github.com/caiguanhao/shopify)

[Shopify GraphQL Admin API Docs](https://shopify.dev/api/admin-graphql)

[Shopify REST Admin API Docs](https://shopify.dev/api/admin-rest)

[Create Apps](https://partners.shopify.com/)

## Usage

```go
import "github.com/caiguanhao/shopify"

src := shopify.Oauth2StaticTokenSource(
	&shopify.Oauth2Token{AccessToken: os.Getenv("SHOPIFY_TOKEN")},
)
httpClient := shopify.Oauth2NewClient(context.Background(), src)
client := shopify.NewClient(os.Getenv("SHOPIFY_SHOP"), httpClient)

// set to true to print request and response body to stderr
client.Debug = false

var customers []struct {
	Id    string `json:"id"`
	Email string `json:"email"`
}
client.New(`query { customers(first: 10) { edges { node { id email } } } }`).
	MustDo(&customers, "customers.edges.*.node")

// get all customers with input parameters and pagination output

customers = nil

var cursor *string
var hasNextPage bool = true
for hasNextPage {
	client.New(`query ($n: Int, $after: String) {
customers(first: $n, after: $after) {
pageInfo { hasNextPage }
edges { cursor node { id email } } } }`, "n", 2, "after", cursor).
		MustDo(
			&customers, "customers.edges.*.node",
			&cursor, "customers.edges.*.cursor",
			&hasNextPage, "customers.pageInfo.hasNextPage",
		)
}

// For Restful API:

// get themes
var themes []struct {
	Id   int
	Role string
}
client.NewRest("GET", "themes").MustDo(&themes, "themes.*")

// get files of a specific theme
var files []string
client.NewRest("GET", fmt.Sprintf("themes/%d/assets", 100000000000), shopify.KV{
	"fields": "key",
}).MustDo(&files, "assets.*.key")

// update theme file
var updatedAt string
client.NewRest("PUT", fmt.Sprintf("themes/%d/assets", 100000000000), nil, shopify.KV{
	"asset": shopify.KV{
		"key":   "layout/theme.liquid",
		"value": "<html>...</html>",
	},
}).MustDo(&updatedAt, "asset.updated_at")
```

### Multiple queries in one request

You can use NewMulti() or write your own GQL:

```go
var launchUrl, currencyCode string
client.New("{ cai: currentAppInstallation { launchUrl }, shop: shop { currencyCode } }").
	MustDo(&launchUrl, "cai.launchUrl", &currencyCode, "shop.currencyCode")
fmt.Println(launchUrl, currencyCode)
```

Below is an example to find multiple discount codes in one request.
[codeDiscountNodeByCode](https://shopify.dev/api/admin-graphql/2021-10/queries/codediscountnodebycode)

```go
package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/caiguanhao/shopify"
)

const (
	// your shop info here, must have read_discounts permission
	shop  = "myshop"
	token = "shpat_00000000000000000000000000000000"
)

var client *shopify.Client = shopify.NewClient(shop, shopify.Oauth2NewClient(
	context.Background(),
	shopify.Oauth2StaticTokenSource(
		&shopify.Oauth2Token{AccessToken: token},
	),
))

func main() {
	discounts, err := findDiscount("CODE1", "CODE2")
	if err != nil {
		panic(err)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	e.Encode(discounts)
}

type (
	shopifyCodeDiscountNodeResponse struct {
		Id       string                      `json:"id"`
		Discount shopifyCodeDiscountResponse `json:"codeDiscount"`
	}

	shopifyCodeDiscountResponse struct {
		TypeName string     `json:"__typename"`
		Title    string     `json:"title"`
		Status   string     `json:"status"`
		StartsAt *time.Time `json:"startsAt"`
		EndsAt   *time.Time `json:"endsAt"`
	}
)

func findDiscount(codes ...string) (discounts []*shopifyCodeDiscountNodeResponse, err error) {
	base := `{ id codeDiscount { __typename ...on DiscountCodeBasic {
  title status startsAt endsAt
} } }`
	//--- use NewMulti:

	gql, args, makeTargets := shopify.NewMulti("query", "codeDiscountNodeByCode",
		"code: String!", base, shopify.Slice(codes)...)
	targets := makeTargets(&discounts, "")

	//--- or use NewMulti with fragment: (this could make gql shorter)

	// gql, args, makeTargets := shopify.NewMulti("query", "codeDiscountNodeByCode",
	// 	"code: String!", "DiscountCodeNode"+base, shopify.Slice(codes)...)
	// targets := makeTargets(&discounts, "")

	//--- instead of this:

	// gql := "query ("
	// for i := range codes {
	// 	if i > 0 {
	// 		gql += ", "
	// 	}
	// 	gql += fmt.Sprintf("$c%d: String!", i)
	// }
	// gql += ") {\n"
	// discounts = make([]*shopifyCodeDiscountNodeResponse, len(codes))
	// var args []interface{}
	// var targets []interface{}
	// for i, code := range codes {
	// 	gql += fmt.Sprintf("c%d: codeDiscountNodeByCode(code: $c%d) %s\n", i, i, base)
	// 	args = append(args, fmt.Sprintf("c%d", i), code)
	// 	targets = append(targets, &discounts[i], fmt.Sprintf("c%d", i))
	// }
	// gql += "}"

	err = client.New(gql, args...).Do(targets...)
	if err != nil {
		discounts = nil
	}
	return
}
```

## Oauth2 Example

Create a new App and put `http://127.0.0.1/hello` to "Allowed redirection URL(s)".

Copy the API key as ClientID, API secret key as ClientSecret.

```go
conf := &shopify.Oauth2Config{
	ClientID:     "f75xxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	ClientSecret: "shpss_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	Scopes: []string{
		"read_customers",
		// for list of access scopes, visit:
		// https://shopify.dev/api/usage/access-scopes
	},
	RedirectURL: "http://127.0.0.1/hello",
	Endpoint: shopify.Oauth2Endpoint{
		AuthURL:  "https://<YOUR-SHOP-NAME>.myshopify.com/admin/oauth/authorize",
		TokenURL: "https://<YOUR-SHOP-NAME>.myshopify.com/admin/oauth/access_token",
	},
}

// redirect user to consent page to ask for permission
fmt.Println(conf.AuthCodeURL("state"))

// once user installed the app, it will redirect to url like this:
// http://127.0.0.1/hello?code=codexxxxxxxxxxxxxxxxxxxxxxxxxxxx&hmac=...
// verify the request then get the token with the code in the url
token, err := conf.Exchange(ctx, "codexxxxxxxxxxxxxxxxxxxxxxxxxxxx")
if err != nil {
	log.Fatal(err)
}

// save the token as json for later use
json.Marshal(token)

// load the token
ctx := context.Background()
token := new(shopify.Oauth2Token)
json.Unmarshal(/* token content */, token)
src := conf.TokenSource(ctx, token)
httpClient := shopify.Oauth2NewClient(ctx, src)
client := shopify.NewClient(/* your shop name */, httpClient)
```

## Test

```
export SHOPIFY_TOKEN=shpat_00000000000000000000000000000000 SHOPIFY_SHOP=demo
export DEBUG=1 # if you want to show more info
go test -v ./...
```
