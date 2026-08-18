package adapter

import "io"

// MaxHTTPBody caps provider usage responses (Cursor / Trae).
const MaxHTTPBody = 8 << 20

func ReadHTTPBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, MaxHTTPBody))
}
