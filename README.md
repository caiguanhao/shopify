# shopify

Shopify GraphQL Admin API client

[Docs of this package](https://pkg.go.dev/github.com/caiguanhao/shopify)

[Shopify GraphQL Admin API Docs](https://shopify.dev/api/admin-graphql)

[Create Apps](https://partners.shopify.com/)

## Usage

```go
import "github.com/caiguanhao/shopify"

src := shopify.Oauth2StaticTokenSource(
	&shopify.Oauth2Token{AccessToken: os.Getenv("SHOPIFY_TOKEN")},
)
httpClient := shopify.Oauth2NewClient(context.Background(), src)
client := shopify.NewClient(os.Getenv("SHOPIFY_SHOP"), httpClient)

var customers []struct {
	Id    string `json:"id"`
	Email string `json:"email"`
}
client.New(`query { customers(first: 10) { edges { node { id email } } } }`).
	MustDo(&customers, "customers.edges.*.node")

// input with parameters
// output with pagination

customers = nil
var cursor string
var hasNextPage bool

client.New(`query ($n: Int) { customers(first: $n) {
pageInfo { hasNextPage }
edges { cursor node { id email } } } }`, "n", 2).
	MustDo(
		&customers, "customers.edges.*.node",
		&cursor, "customers.edges.*.cursor",
		&hasNextPage, "customers.pageInfo.hasNextPage",
	)
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
