package shopify

import (
	"net/http"

	"golang.org/x/oauth2"
)

var (
	Oauth2StaticTokenSource = oauth2.StaticTokenSource
	Oauth2NewClient         = oauth2.NewClient
)

type (
	Oauth2Token     = oauth2.Token
	Oauth2Transport = oauth2.Transport

	transport struct {
		base *Oauth2Transport
	}
)

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	token, err := t.base.Source.Token()
	if err != nil {
		return nil, err
	}
	r.Header.Add("X-Shopify-Access-Token", token.AccessToken)
	return t.base.RoundTrip(r)
}
