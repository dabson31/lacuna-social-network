package config

import (
	"fmt"
	"log"
)

// 	
type oAuthConfig struct {
	Provider string
	AuthURL string
	TokenURL string
	UserInfoURL string
	RedirectURL string
	ClientID string
	ClientSecret string
}

func LoadEnvVariables() (bool, error) {
	if err := validateEnvFormat(); err != nil {
		log.Fatal(fmt.Errorf("Error: There was an issue with processing the .env variables; %v", err))
	}
	
	//todo: add the correct client information here
	github := &oAuthConfig{
		Provider:     "github",
		// ClientID:     getEnv("GITHUB_CLIENT_ID", "GitHubClientID"),
		// ClientSecret: getEnv("GITHUB_CLIENT_SECRET", "GitHubClientSecret"),
		// RedirectURL:  getEnv("GITHUB_CALLBACK_URL", "GitHubCallbackURL", "http://localhost:8080/auth/github/callback"),
		// AuthURL:      "https://github.com/login/oauth/authorize",
		// TokenURL:     "https://github.com/login/oauth/access_token",
		// UserInfoURL:  "https://api.github.com/user",
	}
	
	google := &oAuthConfig{
		Provider:     "google",
		// ClientID:     getEnv("GOOGLE_CLIENT_ID", "GoogleClientId"),
		// ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", "GoogleClientSecret"),
		// RedirectURL:  getEnv("GOOGLE_CALLBACK_URL", "GoogleCallbackURL", "http://localhost:8080/auth/google/callback"),
		// AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		// TokenURL:     "https://oauth2.googleapis.com/token",
		// UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
	}

	return false, nil
}