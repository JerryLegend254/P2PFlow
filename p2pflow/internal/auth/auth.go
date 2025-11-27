package auth

import (
	"golang.org/x/oauth2"
)

type Authenticator struct {
	OAuth any
}

func NewAuthenticator(config *oauth2.Config, state string) Authenticator {
	return Authenticator{
		OAuth: NewOAuthAuthenticator(config, state),
	}
}
