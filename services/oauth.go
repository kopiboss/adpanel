package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"adpanel/config"
)

var googleOAuthConfig *oauth2.Config

func InitGoogleOAuth() {
	googleOAuthConfig = &oauth2.Config{
		ClientID:     config.App.GoogleClientID,
		ClientSecret: config.App.GoogleClientSecret,
		RedirectURL:  config.App.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleAuthURL(state string) string {
	if googleOAuthConfig == nil {
		return ""
	}
	return googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func GoogleExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	if googleOAuthConfig == nil {
		return nil, fmt.Errorf("google oauth not configured")
	}
	return googleOAuthConfig.Exchange(ctx, code)
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

func GetGoogleUserInfo(token *oauth2.Token) (*GoogleUserInfo, error) {
	if googleOAuthConfig == nil {
		return nil, fmt.Errorf("google oauth not configured")
	}

	client := googleOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API error: %s", string(body))
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
