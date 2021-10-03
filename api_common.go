package shopify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

func (client *Client) UploadJSONL(jsonl string) (string, error) {
	return client.UploadJSONLWithContext(context.Background(), jsonl)
}

func (client *Client) UploadJSONLWithContext(ctx context.Context, jsonl string) (key string, err error) {
	var uploadUrl string
	var params []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	err = client.New(
		`mutation {
stagedUploadsCreate(input: {
  resource: BULK_MUTATION_VARIABLES,
  filename: "bulk_op_vars",
  mimeType: "text/jsonl",
  httpMethod: POST
}) {
  userErrors { field message }
  stagedTargets { url parameters { name value } }
} }`,
	).WithContext(ctx).Do(
		&uploadUrl, "stagedUploadsCreate.stagedTargets.*.url",
		&params, "stagedUploadsCreate.stagedTargets.*.parameters.*",
	)
	if err != nil {
		return
	}
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	var keyInParams string
	for _, param := range params {
		if param.Name == "key" {
			keyInParams = param.Value
		}
		if err = writer.WriteField(param.Name, param.Value); err != nil {
			return
		}
	}
	var part io.Writer
	part, err = writer.CreateFormFile("file", "data.jsonl")
	if err != nil {
		return
	}
	if _, err = part.Write([]byte(jsonl)); err != nil {
		return
	}
	if err = writer.Close(); err != nil {
		return
	}
	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, "POST", uploadUrl, body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var res *http.Response
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	res.Body.Close()
	if res.StatusCode != 201 {
		err = fmt.Errorf("response status is not ok: %d", res.StatusCode)
		return
	}
	key = keyInParams
	return
}
