package credstore

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

const keychainService = "whereToken"

type cmdStore struct {
	service string
	run     func(args []string) (string, error)
}

func (k *cmdStore) account(key string) string {
	return "hosted." + key
}

func (k *cmdStore) Get(key string) (string, error) {
	out, err := k.run([]string{"find-generic-password", "-s", k.service, "-a", k.account(key), "-w"})
	if err != nil {
		return "", ErrNotFound
	}
	v := strings.TrimSpace(out)
	if v == "" {
		return "", ErrNotFound
	}
	return v, nil
}

func (k *cmdStore) Set(key, value string) error {
	_, err := k.run([]string{"add-generic-password", "-U", "-s", k.service, "-a", k.account(key), "-w", value})
	return err
}

func (k *cmdStore) Delete(key string) error {
	_, err := k.run([]string{"delete-generic-password", "-s", k.service, "-a", k.account(key)})
	if err != nil {
		return nil
	}
	return nil
}

func runSecurity(args []string) (string, error) {
	cmd := exec.Command("security", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", err
		}
		return "", err
	}
	return string(out), nil
}

func osBackend() Store {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if _, err := exec.LookPath("security"); err != nil {
		return nil
	}
	return &cmdStore{service: keychainService, run: runSecurity}
}

type chain struct {
	os, file Store
}

func (c *chain) Get(key string) (string, error) {
	if c.os != nil {
		if v, err := c.os.Get(key); err == nil {
			return v, nil
		}
	}
	return c.file.Get(key)
}

func (c *chain) Set(key, value string) error {
	if c.os != nil {
		if err := c.os.Set(key, value); err == nil {
			_ = c.file.Delete(key)
			return nil
		}
	}
	return c.file.Set(key, value)
}

func (c *chain) Delete(key string) error {
	if c.os != nil {
		_ = c.os.Delete(key)
	}
	return c.file.Delete(key)
}
