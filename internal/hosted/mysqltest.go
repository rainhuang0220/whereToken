package hosted

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	sharedStore     *Store
	sharedStoreErr  error
	sharedStoreOnce sync.Once
	sharedCleanup   func()
)

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedCleanup != nil {
		sharedCleanup()
	}
	os.Exit(code)
}

func readyStore(t *testing.T) *Store {
	t.Helper()
	sharedStoreOnce.Do(func() {
		sharedStore, sharedCleanup, sharedStoreErr = openTestStore()
	})
	if sharedStoreErr != nil {
		t.Skip(sharedStoreErr.Error())
	}
	if sharedStore == nil {
		t.Skip("mysql test store unavailable")
	}
	t.Cleanup(func() {})
	return sharedStore
}

func openTestStore() (*Store, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("WHERETOKEN_MYSQL_TEST_DSN"))
	cleanup := func() {}
	if dsn == "" {
		var err error
		dsn, cleanup, err = startDockerMySQL()
		if err != nil {
			return nil, nil, err
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		if ctx.Err() != nil {
			cleanup()
			return nil, nil, fmt.Errorf("mysql ping: %w", err)
		}
		time.Sleep(400 * time.Millisecond)
	}
	st := &Store{db: db}
	if err := st.Migrate(context.Background()); err != nil {
		cleanup()
		return nil, nil, err
	}
	return st, func() {
		db.Close()
		cleanup()
	}, nil
}

func startDockerMySQL() (string, func(), error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", nil, fmt.Errorf("docker unavailable: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	name := fmt.Sprintf("wheretoken-mysql-%d", port)
	cmd := exec.Command("docker", "run", "-d", "--rm", "--name", name,
		"-e", "MYSQL_ROOT_PASSWORD=wttest",
		"-e", "MYSQL_DATABASE=wheretoken",
		"-p", fmt.Sprintf("127.0.0.1:%d:3306", port),
		"mysql:8.0",
		"--default-authentication-plugin=mysql_native_password",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("docker run mysql: %w (%s)", err, bytesPreview(out))
	}
	cleanup := func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	}
	dsn := fmt.Sprintf("root:wttest@tcp(127.0.0.1:%d)/wheretoken?parseTime=true&charset=utf8mb4", port)
	return dsn, cleanup, nil
}

func bytesPreview(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
