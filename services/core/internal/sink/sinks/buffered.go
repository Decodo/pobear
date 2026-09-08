package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

var errClosed = errors.New("sinks: write after close")

func marshalJSONNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func writeJSONLine(w io.Writer, item Item) error {
	data, err := marshalJSONNoEscape(item)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func bufferedWrite(closed *bool, items *[]Item, item Item) error {
	if *closed {
		return errClosed
	}
	*items = append(*items, item)
	return nil
}

func bufferedClose(closed *bool, path string, items []Item, marshal func(any) ([]byte, error)) error {
	if *closed {
		return nil
	}
	*closed = true

	if items == nil {
		items = []Item{}
	}
	data, err := marshal(items)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) //nolint:gosec // path is the caller-supplied sink target, not user-controlled input
}
