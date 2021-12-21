package shopify

import (
	"context"
	"testing"
	"time"
)

func TestNewMulti(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ids, titles [][]string
	var codes []string
	var app map[string]string

	gql, args, targets := NewMulti("query").
		Add("products", "first: Int, reverse: Boolean").
		Return("ProductConnection { edges { node { id title } } }"). // with fragment
		In(3, true, 3, false).
		Out(&ids, ".edges.*.node.id", &titles, ".edges.*.node.title").
		Self().
		Add("codeDiscountNodes", "first: Int").
		Return("{ edges { node { codeDiscount { ...on DiscountCodeBasic { title } } } } }"). // without fragment
		In(10).
		Out(&codes, ".edges.*.node.codeDiscount.title").
		Self().
		Add("currentAppInstallation").
		Return(`AppInstallation { id launchUrl }`). // with fragment
		Out(&app, "").
		Self().
		Do()

	if gql != `fragment frag0 on ProductConnection { edges { node { id title } } }
fragment frag1 on AppInstallation { id launchUrl }
query($first0: Int, $reverse0: Boolean, $first1: Int, $reverse1: Boolean, $first2: Int) {
gql0: products(first: $first0, reverse: $reverse0) { ...frag0 }
gql1: products(first: $first1, reverse: $reverse1) { ...frag0 }
gql2: codeDiscountNodes(first: $first2) { edges { node { codeDiscount { ...on DiscountCodeBasic { title } } } } }
gql3: currentAppInstallation { ...frag1 }
}` {
		t.Error("gql is not correct")
	}
	if toJSON(args) != `["first0",3,"reverse0",true,"first1",3,"reverse1",false,"first2",10]` {
		t.Error("args are not correct")
	}
	if toJSON(targets) != `[null,"gql0.edges.*.node.id",null,"gql1.edges.*.node.id",null,"gql0.edges.*.node.title",null,"gql1.edges.*.node.title",null,"gql2.edges.*.node.codeDiscount.title",null,"gql3"]` {
		t.Error("targets are not correct")
	}
	client.New(gql, args...).WithContext(ctx).MustDo(targets...)
	t.Log("ids =", ids)
	t.Log("titles =", titles)
	t.Log("app =", app)
	if len(ids) < 1 {
		t.Error("ids should not be empty")
	}
	if len(titles) < 1 {
		t.Error("titles should not be empty")
	}
	if len(app) < 1 {
		t.Error("app should not be empty")
	}
	presentCodes := RemoveZeroValues(codes)
	if len(presentCodes) < 1 {
		t.Log("warning: no discount codes found")
		return
	}
	t.Log("codes =", presentCodes)
}
