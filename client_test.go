package shopify

import (
	"context"
	"encoding/json"
	"fmt"
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
	client.Debug = os.Getenv("DEBUG") == "1"
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

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

func TestNull(t *testing.T) {
	var resp1 []string
	var resp2 []*string
	var resp3 string
	var resp4 *string
	client.New(`query ($ids: [ID!]!) { nodes(ids: $ids) { ...on ProductVariant { id } } }`,
		"ids", []string{"gid://shopify/ProductVariant/1"}).
		MustDo(
			&resp1, "nodes.*.id", &resp2, "nodes.*.id",
			&resp3, "nodes.*.id", &resp4, "nodes.*.id",
		)
	if len(resp1) != 1 {
		t.Error("ERROR: length should be 1")
	} else {
		if resp1[0] != "" {
			t.Error("ERROR: string must be empty")
		}
	}
	if len(resp2) != 1 {
		t.Error("ERROR: length should be 1")
	} else {
		if resp2[0] != nil {
			t.Error("ERROR: string must be nil")
		}
	}
	if resp3 != "" {
		t.Error("ERROR: string must be empty")
	}
	if resp4 != nil {
		t.Error("ERROR: string must be nil")
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

func TestNewMulti(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// with fragment:

	gql, args, targets := NewMulti(
		"query",
		"products",
		"first: Int, reverse: Boolean",
		`ProductConnection { edges { node { id title } } }`,
		1, true, 1, false,
	)
	if gql != `fragment FRAG on ProductConnection { edges { node { id title } } }
query ($first0: Int, $reverse0: Boolean, $first1: Int, $reverse1: Boolean) {
gql0: products(first: $first0, reverse: $reverse0) { ...FRAG }
gql1: products(first: $first1, reverse: $reverse1) { ...FRAG }
}` {
		t.Error("gql is not correct")
	}
	if toJSON(args) != `["first0",1,"reverse0",true,"first1",1,"reverse1",false]` {
		t.Error("args are not correct")
	}
	var ids, titles []string
	dests := targets(&ids, ".edges.*.node.id", &titles, ".edges.*.node.title")
	if toJSON(dests) != `["","gql0.edges.*.node.id","","gql1.edges.*.node.id","","gql0.edges.*.node.title","","gql1.edges.*.node.title"]` {
		t.Error("targets are not correct")
	}
	client.New(gql, args...).WithContext(ctx).MustDo(dests...)
	t.Log("ids =", ids)
	t.Log("titles =", titles)
	if len(ids) < 1 {
		t.Error("ids should not be empty")
	}
	if len(titles) < 1 {
		t.Error("titles should not be empty")
	}

	var codes []string
	client.New(`{ codeDiscountNodes(first: 10) { edges { node {
codeDiscount { ...on DiscountCodeBasic { title } } } } } }`).WithContext(ctx).MustDo(
		&codes, "codeDiscountNodes.edges.*.node.codeDiscount.title",
	)
	presentCodes := RemoveZeroValues(codes)
	if len(presentCodes) < 2 {
		t.Log("warning: no discount codes to test")
		return
	}
	presentCodes = presentCodes[0:2]
	t.Log("codes =", presentCodes)

	// without fragment:

	gql, args, targets = NewMulti(
		"query",
		"codeDiscountNodeByCode",
		"code: String!",
		`{ id codeDiscount { ...on DiscountCodeBasic { status startsAt endsAt } } }`,
		Slice(presentCodes)...,
	)
	if gql != `query ($code0: String!, $code1: String!) {
gql0: codeDiscountNodeByCode(code: $code0) { id codeDiscount { ...on DiscountCodeBasic { status startsAt endsAt } } }
gql1: codeDiscountNodeByCode(code: $code1) { id codeDiscount { ...on DiscountCodeBasic { status startsAt endsAt } } }
}` {
		t.Error("gql is not correct")
	}
	if toJSON(args) != fmt.Sprintf(`["code0","%s","code1","%s"]`, presentCodes[0], presentCodes[1]) {
		t.Error("args are not correct")
	}
	var codeIds []string
	var codeDiscounts []struct {
		Status   string     `json:"status"`
		StartsAt *time.Time `json:"startsAt"`
		EndsAt   *time.Time `json:"endsAt"`
	}
	dests = targets(&codeIds, ".id", &codeDiscounts, ".codeDiscount")
	client.New(gql, args...).WithContext(ctx).MustDo(dests...)
	t.Log("code ids =", codeIds)
	t.Log("code discounts =", codeDiscounts)
	if len(codeIds) != 2 {
		t.Error("code ids are not correct")
	}
	if len(codeDiscounts) != 2 {
		t.Error("code discounts are not correct")
	}
}

func TestSlice(t *testing.T) {
	if toJSON(Slice([]string{"a", "b"})) != `["a","b"]` {
		t.Error("incorrect Slice()")
	}
	if toJSON(Slice([]int{2, 0, 1})) != `[2,0,1]` {
		t.Error("incorrect Slice()")
	}
}

func TestRemoveZeroValues(t *testing.T) {
	if toJSON(RemoveZeroValues([]string{"a", "", "b"})) != `["a","b"]` {
		t.Error("incorrect RemoveZeroValues()")
	}
	if toJSON(RemoveZeroValues([]int{0, 1, 0, 3, 0})) != `[1,3]` {
		t.Error("incorrect RemoveZeroValues()")
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

func toJSON(i interface{}) string {
	b, _ := json.Marshal(i)
	return string(b)
}
