package telegram

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// terminalAuth implements auth.UserAuthenticator by prompting the user via terminal.
type terminalAuth struct {
	phone    string
	password string
	reader   io.Reader
}

func (a terminalAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a terminalAuth) Password(_ context.Context) (string, error) {
	if a.password != "" {
		return a.password, nil
	}
	fmt.Print("Enter 2FA password: ")
	scanner := bufio.NewScanner(a.reader)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", fmt.Errorf("failed to read 2FA password")
}

func (a terminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter OTP code: ")
	scanner := bufio.NewScanner(a.reader)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", fmt.Errorf("failed to read OTP code")
}

func (a terminalAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (a terminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported — please register via official Telegram app")
}

// Authenticate runs the gotd auth flow if not already authorized.
// It prompts the user for phone number, OTP code, and optionally 2FA password.
// On success it prints the authenticated user info.
func (c *Client) Authenticate(ctx context.Context, phone string, password string) error {
	// Check if already authorized.
	status, err := c.client.Auth().Status(ctx)
	if err != nil {
		return fmt.Errorf("check auth status: %w", err)
	}

	if status.Authorized {
		user, err := c.GetSelf(ctx)
		if err != nil {
			return fmt.Errorf("get self after existing auth: %w", err)
		}
		printAuthSuccess(user)
		return nil
	}

	// Run the auth flow.
	flow := auth.NewFlow(
		&terminalAuth{
			phone:    phone,
			password: password,
			reader:   os.Stdin,
		},
		auth.SendCodeOptions{},
	)

	if err := flow.Run(ctx, c.client.Auth()); err != nil {
		return fmt.Errorf("auth flow: %w", err)
	}

	// Validate by fetching self.
	user, err := c.GetSelf(ctx)
	if err != nil {
		return fmt.Errorf("get self after auth: %w", err)
	}

	printAuthSuccess(user)
	return nil
}

// IsAuthorized checks whether the current session is still valid.
func (c *Client) IsAuthorized(ctx context.Context) (bool, error) {
	status, err := c.client.Auth().Status(ctx)
	if err != nil {
		return false, err
	}
	return status.Authorized, nil
}

// printAuthSuccess prints the authenticated user's information.
func printAuthSuccess(user *tg.User) {
	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if user.Username != "" {
		fmt.Printf("Authenticated as %s (@%s)\n", name, user.Username)
	} else {
		fmt.Printf("Authenticated as %s\n", name)
	}
}
