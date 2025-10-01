package auth

import (
	"fmt"
	"math/rand"

	"golang.org/x/oauth2"
)

type OAuthAuthenticator struct {
	config *oauth2.Config
	state  string
}

func NewOAuthAuthenticator(config *oauth2.Config, state string) *OAuthAuthenticator {
	return &OAuthAuthenticator{
		config: config,
		state:  state, // TODO: Use a dynamic state in production
	}
}

func GenerateRandomState() string {
	return fmt.Sprintf("%d", rand.Intn(100000))
}
