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
	Short: "Manage authentication",
	Long:  `Authenticate with GitHub, GitLab, or other platforms to access repositories and perform operations`,
}

// authLoginCmd handles OAuth login
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a platform",
	Long:  `Start the OAuth device flow to authenticate with GitHub or GitLab and obtain an access token`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitlab, _ := cmd.Flags().GetBool("gitlab")

		platform := auth.PlatformGitHub
		if gitlab {
			platform = auth.PlatformGitLab
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		return auth.Login(ctx, platform)
	},
}

// authLogoutCmd handles logout
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of a platform",
	Long:  `Remove stored authentication credentials for GitHub or GitLab`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitlab, _ := cmd.Flags().GetBool("gitlab")

		platform := auth.PlatformGitHub
		if gitlab {
			platform = auth.PlatformGitLab
		}

		return auth.Logout(platform)
	},
}

// authStatusCmd shows authentication status
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "View authentication status",
	Long:  `Display current authentication status and user information for GitHub and GitLab`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitlab, _ := cmd.Flags().GetBool("gitlab")

		// If --gitlab flag is set, show only GitLab status
		if gitlab {
			return showPlatformStatus(auth.PlatformGitLab)
		}

		// Otherwise, show status for all platforms
		fmt.Println("Authentication Status")
		fmt.Println("====================")

		// GitHub status
		fmt.Println("\nGitHub:")
		ghAuthMethod := auth.GetAuthMethod(auth.PlatformGitHub)
		if ghAuthMethod == nil {
			fmt.Println("  Not authenticated")
			fmt.Println("  To authenticate: ivaldi auth login")
		} else {
			fmt.Printf("  %s\n", ghAuthMethod.Description)
			user, err := getGitHubUser(ghAuthMethod.Token)
			if err != nil {
				fmt.Printf("  Token may be invalid: %v\n", err)
			} else {
				fmt.Printf("  Logged in as: %s\n", user.Login)
			}
		}

		// GitLab status
		fmt.Println("\nGitLab:")
		glAuthMethod := auth.GetAuthMethod(auth.PlatformGitLab)
		if glAuthMethod == nil {
			fmt.Println("  Not authenticated")
			fmt.Println("  To authenticate: ivaldi auth login --gitlab")
		} else {
			fmt.Printf("  %s\n", glAuthMethod.Description)
			user, err := getGitLabUser(glAuthMethod.Token)
			if err != nil {
				fmt.Printf("  Token may be invalid: %v\n", err)
			} else {
				fmt.Printf("  Logged in as: %s\n", user.Username)
			}
		}

		return nil
	},
}

// showPlatformStatus shows authentication status for a specific platform
func showPlatformStatus(platform auth.Platform) error {
	platformName := string(platform)
	authMethod := auth.GetAuthMethod(platform)

	if authMethod == nil {
		fmt.Printf("Not authenticated with %s\n", platformName)
		fmt.Println("\nTo authenticate, run:")
		if platform == auth.PlatformGitLab {
			fmt.Println("  ivaldi auth login --gitlab")
		} else {
			fmt.Println("  ivaldi auth login")
		}
		fmt.Println("\nAlternatively, you can:")
		if platform == auth.PlatformGitHub {
			fmt.Println("  - Set GITHUB_TOKEN environment variable")
			fmt.Println("  - Use 'gh auth login' (GitHub CLI)")
		} else {
			fmt.Println("  - Set GITLAB_TOKEN environment variable")
			fmt.Println("  - Use 'glab auth login' (GitLab CLI)")
		}
		fmt.Println("  - Configure git credentials")
		return nil
	}

	// Display authentication method
	fmt.Printf("%s\n", authMethod.Description)

	// Test the token by making a request
	if platform == auth.PlatformGitHub {
		user, err := getGitHubUser(authMethod.Token)
		if err != nil {
			fmt.Println("\nAuthenticated, but token may be invalid")
			fmt.Printf("Error: %v\n", err)
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
	} else {
		user, err := getGitLabUser(authMethod.Token)
		if err != nil {
			fmt.Println("\nAuthenticated, but token may be invalid")
			fmt.Printf("Error: %v\n", err)
			return nil
		}

		fmt.Printf("\nLogged in to GitLab as: %s\n", user.Username)
		if user.Name != "" {
			fmt.Printf("Name: %s\n", user.Name)
		}
		if user.Email != "" {
			fmt.Printf("Email: %s\n", user.Email)
		}
	}

	// Show additional info based on auth method
	if authMethod.Name != "ivaldi" {
		fmt.Println("\nNote: You're using an external authentication method.")
		fmt.Println("To use Ivaldi's built-in OAuth, run:")
		if platform == auth.PlatformGitLab {
			fmt.Println("  ivaldi auth login --gitlab")
		} else {
			fmt.Println("  ivaldi auth login")
		}
	}

	return nil
}

// GitHubUser represents a GitHub user
type GitHubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}

// GitLabUser represents a GitLab user
type GitLabUser struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// getGitHubUser fetches the authenticated GitHub user's information
func getGitHubUser(token string) (*GitHubUser, error) {
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

// getGitLabUser fetches the authenticated GitLab user's information
func getGitLabUser(token string) (*GitLabUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://gitlab.com/api/v4/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var user GitLabUser
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

	// Add --gitlab flag to auth commands
	authLoginCmd.Flags().Bool("gitlab", false, "Authenticate with GitLab instead of GitHub")
	authLogoutCmd.Flags().Bool("gitlab", false, "Log out from GitLab instead of GitHub")
	authStatusCmd.Flags().Bool("gitlab", false, "Show only GitLab authentication status")
}
