package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/auth"
	"github.com/spf13/cobra"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage GitHub authentication",
	Long:  `Authenticate with GitHub to access repositories and perform operations`,
}

// authLoginCmd handles OAuth login
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with GitHub",
	Long:  `Start the OAuth device flow to authenticate with GitHub and obtain an access token`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		return auth.Login(ctx)
	},
}

// authLogoutCmd handles logout
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of GitHub",
	Long:  `Remove stored GitHub authentication credentials`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return auth.Logout()
	},
}

// authStatusCmd shows authentication status
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "View authentication status",
	Long:  `Display current GitHub authentication status and user information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check authentication method
		authMethod := auth.GetAuthMethod()

		if authMethod == nil {
			fmt.Println("Not authenticated with GitHub")
			fmt.Println("\nTo authenticate, run:")
			fmt.Println("  ivaldi auth login")
			fmt.Println("\nAlternatively, you can:")
			fmt.Println("  - Set GITHUB_TOKEN environment variable")
			fmt.Println("  - Use 'gh auth login' (GitHub CLI)")
			fmt.Println("  - Configure git credentials")
			return nil
		}

		// Display authentication method
		fmt.Printf("%s\n", authMethod.Description)

		// Test the token by making a request to GitHub
		user, err := getAuthenticatedUser(authMethod.Token)
		if err != nil {
			fmt.Println("\nAuthenticated, but token may be invalid")
			fmt.Printf("Error: %v\n", err)

			if authMethod.Name == "ivaldi" {
				fmt.Println("\nTry logging in again:")
				fmt.Println("  ivaldi auth login")
			} else if authMethod.Name == "gh-cli" {
				fmt.Println("\nTry re-authenticating with GitHub CLI:")
				fmt.Println("  gh auth login")
			} else {
				fmt.Println("\nCheck your authentication credentials or use:")
				fmt.Println("  ivaldi auth login")
			}
			return nil
		}

		fmt.Printf("\nLogged in to GitHub as: %s\n", user.Login)
		if user.Name != "" {
			fmt.Printf("Name: %s\n", user.Name)
		}
		if user.Email != "" {
			fmt.Printf("Email: %s\n", user.Email)
		}
		fmt.Printf("Account type: %s\n", user.Type)

		// Show additional info based on auth method
		if authMethod.Name != "ivaldi" {
			fmt.Println("\nNote: You're using an external authentication method.")
			fmt.Println("To use Ivaldi's built-in OAuth, run:")
			fmt.Println("  ivaldi auth login")
		}

		return nil
	},
}

// GitHubUser represents a GitHub user
type GitHubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}

// getAuthenticatedUser fetches the authenticated user's information
func getAuthenticatedUser(token string) (*GitHubUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}
