# shopify

Shopify GraphQL Admin API client

Usage:

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
