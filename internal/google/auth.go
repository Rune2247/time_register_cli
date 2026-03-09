package google

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var scopes = []string{
	sheets.SpreadsheetsScope,
	calendar.CalendarEventsScope,
}

// tokenPath returns the path to the stored OAuth token.
func tokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "timereg", "token.json"), nil
}

// credentialsPath returns the path to the OAuth credentials file.
func credentialsPath() (string, error) {
	return CredentialsFilePath()
}

// CredentialsFilePath returns the expected path for credentials.json.
func CredentialsFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "timereg", "credentials.json"), nil
}

// CredentialsFileExists checks if the credentials.json file exists.
func CredentialsFileExists() bool {
	p, err := CredentialsFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// getOAuthConfig reads the credentials.json and returns an OAuth2 config.
func getOAuthConfig() (*oauth2.Config, error) {
	credPath, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file at %s: %w\n\nTo set up Google integration:\n1. Go to https://console.cloud.google.com\n2. Create a project and enable Sheets + Calendar APIs\n3. Create OAuth2 credentials (Desktop app)\n4. Download credentials.json to %s", credPath, err, credPath)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	return config, nil
}

// loadToken reads the saved OAuth token from disk.
func loadToken() (*oauth2.Token, error) {
	tokPath, err := tokenPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(tokPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves an OAuth token to disk.
func saveToken(token *oauth2.Token) error {
	tokPath, err := tokenPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(tokPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to save token: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

// Authenticate runs the OAuth2 flow if needed and returns an authenticated HTTP client option.
func Authenticate(ctx context.Context) (option.ClientOption, error) {
	config, err := getOAuthConfig()
	if err != nil {
		return nil, err
	}

	tok, err := loadToken()
	if err != nil {
		// No saved token — run the auth flow
		tok, err = runAuthFlow(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tok); err != nil {
			return nil, err
		}
	}

	client := config.Client(ctx, tok)
	return option.WithHTTPClient(client), nil
}

// runAuthFlow opens a browser-based OAuth flow and returns the token.
func runAuthFlow(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following URL in your browser:\n\n%s\n\nEnter the authorization code: ", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}

	return tok, nil
}

// RunAuthSetup forces re-authentication (for timereg config setup).
func RunAuthSetup() error {
	config, err := getOAuthConfig()
	if err != nil {
		return err
	}

	tok, err := runAuthFlow(config)
	if err != nil {
		return err
	}

	if err := saveToken(tok); err != nil {
		return err
	}

	fmt.Println("Authentication successful! Token saved.")
	return nil
}

// IsAuthenticated checks if a valid token exists.
func IsAuthenticated() bool {
	_, err := loadToken()
	return err == nil
}
