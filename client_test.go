package shopify

import (
	"context"
	"os"
	"testing"
	"time"
)

var client *Client

func init() {
	src := Oauth2StaticTokenSource(
		&Oauth2Token{AccessToken: os.Getenv("SHOPIFY_TOKEN")},
	)
	httpClient := Oauth2NewClient(context.Background(), src)
	client = NewClient(os.Getenv("SHOPIFY_SHOP"), httpClient)
}

func TestQuery(t *testing.T) {
	const gql = `
query ($n: Int, $cursor: String) { customers(first: $n, after: $cursor) {
pageInfo { hasNextPage }
edges { cursor node { id } }
} }`

	var results []struct {
		Id string `json:"id"`
	}
	var cursor string
	var hasNextPage bool

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)

	client.New(gql, "n", 3).WithContext(ctx).MustDo(
		&results, "customers.edges.*.node",
		&cursor, "customers.edges.*.cursor",
		&hasNextPage, "customers.pageInfo.hasNextPage",
	)

	t.Logf("results: %+v", results)
	t.Logf("cursor: %+v", cursor)
	t.Logf("hasNextPage: %+v", hasNextPage)

	if len(results) != 3 {
		t.Errorf("ERROR: length should be %d instead of %d", 3, len(results))
	}
	if cursor == "" {
		t.Error("ERROR: cursor should not be empty")
	}
	if !hasNextPage {
		t.Error("ERROR: it should have next page")
	}
}

func TestMutation(t *testing.T) {
	const example = "https://example.com/"

	const q = `
query ($query: URL) { scriptTags(first: 10, src: $query) { edges { node { id src } } } }
`
	var ids []string
	client.New(q, "query", example).MustDo(&ids, "scriptTags.edges.*.node.id")
	t.Log("existing ids:", ids)

	for _, id := range ids {
		deleteScriptTag(t, id)
	}

	const cm = `
mutation ($input: ScriptTagInput!) { scriptTagCreate(input: $input) {
userErrors { field message } scriptTag { id } } }
`
	err := client.New(cm, "input", map[string]string{
		"displayScope": "ALL",
		"src":          "http://test-user-error",
	}).Do()
	if err == nil || err.Error() != "Source must be secure (HTTPS)" {
		t.Error("ERROR: wrong user error returned")
	}

	var id string
	client.New(cm, "input", map[string]string{
		"displayScope": "ALL",
		"src":          "https://example.com",
	}).MustDo(&id, "scriptTagCreate.scriptTag.id")
	if id == "" {
		t.Error("ERROR: id should not be empty")
	} else {
		t.Log("created", id)
		deleteScriptTag(t, id)
	}
}

func deleteScriptTag(t *testing.T, id string) {
	const gql = `
mutation ($id: ID!) { scriptTagDelete(id: $id) {
userErrors { field message } deletedScriptTagId } }
`
	var deleted string
	client.New(gql, "id", id).MustDo(&deleted, "scriptTagDelete.deletedScriptTagId")
	if deleted == id {
		t.Log("deleted", id)
	} else {
		t.Error("ERROR: wrong deleted id")
	}
}
