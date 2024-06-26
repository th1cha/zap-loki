package zaploki

import "net/http"

type Authenticator interface {
	Apply(req *http.Request)
}

type BasicAuthenticator struct {
	Username string
	Password string
}

func (b *BasicAuthenticator) Apply(req *http.Request) {
	req.SetBasicAuth(b.Username, b.Password)
}

type APIKeyAuthenticator struct {
	KeyName string
	APIKey  string
}

func (a *APIKeyAuthenticator) Apply(req *http.Request) {
	req.Header.Set(a.KeyName, a.APIKey)
}
