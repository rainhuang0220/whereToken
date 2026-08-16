package adapter

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

func SQLiteURI(path, query string) string {
	p := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && len(p) >= 2 && p[1] == ':' {
		p = "/" + p
	}
	p = strings.ReplaceAll(p, "%", "%25")
	p = strings.ReplaceAll(p, "?", "%3F")
	p = strings.ReplaceAll(p, "#", "%23")
	if query == "" {
		return "file:" + p
	}
	return "file:" + p + "?" + query
}

func OpenRO(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", SQLiteURI(path, "mode=ro&immutable=1"))
	if err == nil {
		if pingErr := db.Ping(); pingErr == nil {
			return db, nil
		}
		db.Close()
	}
	db, err = sql.Open("sqlite", SQLiteURI(path, "mode=ro"))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
