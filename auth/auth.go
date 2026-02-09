package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/emersion/go-imap/client" // <--- v1 Import
	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Username struct {
	value string
}

func NewUser() *Username {
	return &Username{
		value: "",
	}
}

func (u *Username) SetUserName() error {
	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	userInput := strings.TrimSpace(input)
	if !strings.Contains(userInput, "@gmail.com") {
		log.Fatalf("Must contain an `@gmail.com`")
	}
	u.value = userInput
	return nil
}

const tokenFile = "token.json"

// Authenticate using OAuth2 with credentials.json
// Returns a v1 *client.Client
func Authenticate() (*client.Client, error) {
	// 1. Read credentials.json
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials.json: %w", err)
	}

	// 2. Parse the credentials
	config, err := google.ConfigFromJSON(b, "https://mail.google.com/")
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	// 3. Get token (either from file or new auth)
	token := getToken(config)

	// 4. Connect to Gmail IMAP (v1 Style)
	fmt.Println("Connecting to Gmail...")

	// In v1, we Dial directly from the client package
	c, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// 5. Authenticate with OAuth2
	// We use the XOAUTH2 mechanism via the SASL library
	fmt.Println("Authenticating with OAuth2...")

	saslClient := sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{
		Username: "nko3@njit.edu", // <--- Ensure this matches the authenticated user
		Token:    token.AccessToken,
	})

	// v1 Authenticate takes the SASL client directly
	if err := c.Authenticate(saslClient); err != nil {
		c.Logout()
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println("✓ Authenticated successfully")
	return c, nil
}

// getToken retrieves a token from file or initiates OAuth flow
func getToken(config *oauth2.Config) *oauth2.Token {
	token, err := tokenFromFile(tokenFile)
	if err == nil {
		return token
	}

	token = getTokenFromWeb(config)
	saveToken(tokenFile, token)
	return token
}

// getTokenFromWeb uses OAuth2 flow with local server to get token
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	// Add localhost callback to config
	config.RedirectURL = "http://localhost:8080/callback"

	codeChan := make(chan string)
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Code not found", http.StatusBadRequest)
			return
		}
		codeChan <- code
		fmt.Fprintf(w, "Authentication successful! You can close this window.")
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Generate auth URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Opening browser for authentication...\n")
	fmt.Printf("If browser doesn't open, go to:\n%v\n\n", authURL)

	openBrowser(authURL)

	authCode := <-codeChan
	server.Shutdown(context.Background())

	token, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token: %v", err)
	}

	return token
}

// openBrowser tries to open the URL in default browser
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Println("Could not open browser automatically. Please use the link above.")
	}
}

// tokenFromFile retrieves token from local file
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves token to file
func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("Unable to cache token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
