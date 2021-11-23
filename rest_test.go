package shopify

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRestRequest(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	var themes []struct {
		Id   int
		Role string
	}
	client.NewRest("GET", "themes").WithContext(ctx).
		MustDo(&themes, "themes.*")
	var mainThemeId int
	for _, theme := range themes {
		if theme.Role == "main" {
			mainThemeId = theme.Id
		}
	}
	if mainThemeId == 0 {
		t.Log("no theme to test")
	}
	var files []string
	client.NewRest("GET", fmt.Sprintf("themes/%d/assets", mainThemeId), KV{
		"fields": "key",
	}).WithContext(ctx).MustDo(&files, "assets.*.key")
	var layoutTheme string
	for _, file := range files {
		if strings.Contains(file, "layout") && strings.Contains(file, "theme") {
			layoutTheme = file
		}
	}
	if layoutTheme == "" {
		t.Log("no layout file to test")
	}
	var content string
	client.NewRest("GET", fmt.Sprintf("themes/%d/assets", mainThemeId), KV{
		"asset[key]": layoutTheme,
	}).WithContext(ctx).MustDo(&content, "asset.value")

	var a, b string
	i := strings.Index(content, "window.TESTED_AT")
	if i > -1 {
		j := strings.LastIndex(content[0:i], "\n")
		if j > -1 {
			a = content[0:j]
		}
		k := strings.Index(content[i:], "\n")
		if k > -1 {
			b = content[i+k:]
		}
		content = a + b
	}
	i = strings.Index(content, "</head>")
	if i > -1 {
		add := `  {% if customer %}<script>window.TESTED_AT = "` + time.Now().Format(time.RFC3339) + `";</script>{% endif %}`
		content = content[0:i] + add + "\n" + content[i:]
	}

	var updatedAt string
	client.NewRest("PUT", fmt.Sprintf("themes/%d/assets", mainThemeId), nil, KV{
		"asset": KV{
			"key":   layoutTheme,
			"value": content,
		},
	}).WithContext(ctx).MustDo(&updatedAt, "asset.updated_at")
	t.Log("updated at", updatedAt)
}
