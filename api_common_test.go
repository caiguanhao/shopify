package shopify

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBulk(t *testing.T) {
	var products []struct {
		Id    string `json:"id"`
		Title string `json:"title"`
	}
	client.New(`query { products(first: 1) { edges { node { id title } } } }`).
		MustDo(&products, "products.edges.*.node")
	t.Log("Products:", products)

	var jsonl []string
	for _, product := range products {
		title := product.Title
		if strings.ToUpper(title) == title {
			title = strings.ToLower(title)
		} else {
			title = strings.ToUpper(title)
		}
		b, _ := json.Marshal(struct {
			Id    string `json:"id"`
			Title string `json:"title"`
		}{product.Id, title})
		jsonl = append(jsonl, fmt.Sprintf(`{"input":%s}`, string(b)))
	}
	jsonlStr := strings.Join(jsonl, "\n")

	key, err := client.UploadJSONL(jsonlStr)
	if err != nil {
		panic(err)
	}
	t.Log("Uploaded:", key)

	var operation struct {
		Id             string `json:"id"`
		Status         string `json:"status"`
		ErrorCode      string `json:"errorCode"`
		Url            string `json:"url"`
		PartialDataUrl string `json:"partialDataUrl"`
	}

	const bm = `mutation ($mutation: String!, $path: String!) {
bulkOperationRunMutation(mutation: $mutation, stagedUploadPath: $path) {
userErrors { field message }
bulkOperation { id status }
} }`

	client.New(bm,
		"mutation", `mutation ($input: ProductInput!) {
productUpdate(input: $input) { userErrors { field message } product { id title } } }`,
		"path", key,
	).MustDo(
		&operation, "bulkOperationRunMutation.bulkOperation",
	)
	t.Log(operation.Status, operation.Id)

	for {
		client.New(`query { currentBulkOperation(type: MUTATION) {
id status url errorCode partialDataUrl } }`).MustDo(
			&operation, "currentBulkOperation",
		)
		t.Log(operation.Status, operation.Id)
		url := operation.Url
		if url == "" {
			url = operation.PartialDataUrl
		}
		if url != "" {
			if resp, err := http.Get(url); err == nil {
				b, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				t.Log("Result:", string(b))
			} else {
				t.Log("Url:", url)
			}
			break
		}
		s := operation.Status
		if s == "CANCELED" || s == "COMPLETED" || s == "EXPIRED" || s == "FAILED" {
			break
		}
		time.Sleep(1 * time.Second)
	}
}
