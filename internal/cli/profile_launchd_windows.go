//go:build windows

package cli

import "errors"

func (a *App) execLaunchctl(args ...string) (string, error) {
	return "", errors.New("launchctl is not available")
}
