package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// LogLine is one line of container output.
type LogLine struct {
	T string `json:"t"` // RFC 3339 timestamp from the daemon
	S string `json:"s"` // "out" or "err"
	L string `json:"l"`
}

const maxLine = 16 << 10

// Logs reads a container's output and calls fn per line until the stream ends,
// ctx is cancelled, or fn returns an error. tty containers send one raw stream;
// others are multiplexed in 8-byte-header frames.
func (c *Client) Logs(ctx context.Context, id string, tail string, since string, follow, tty bool, fn func(LogLine) error) error {
	q := url.Values{"stdout": {"1"}, "stderr": {"1"}, "timestamps": {"1"}, "tail": {tail}}
	if follow {
		q.Set("follow", "1")
	}
	if since != "" {
		q.Set("since", since)
	}
	resp, err := c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/logs", q, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	emit := func(stream string, raw []byte) error {
		line := strings.TrimRight(string(raw), "\r")
		ts, rest, ok := strings.Cut(line, " ")
		if !ok || len(ts) < 20 || ts[4] != '-' {
			ts, rest = "", line
		}
		if len(rest) > maxLine {
			rest = rest[:maxLine] + " …"
		}
		return fn(LogLine{T: ts, S: stream, L: rest})
	}
	if tty {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			if err := emit("out", sc.Bytes()); err != nil {
				return err
			}
		}
		return ignoreClosed(ctx, sc.Err())
	}
	return demux(resp.Body, func(stream byte, line []byte) error {
		s := "out"
		if stream == 2 {
			s = "err"
		}
		return emit(s, line)
	})
}

// demux splits Docker's multiplexed stream into lines per stream.
func demux(r io.Reader, fn func(stream byte, line []byte) error) error {
	br := bufio.NewReaderSize(r, 32<<10)
	var hdr [8]byte
	pending := map[byte]*bytes.Buffer{1: {}, 2: {}}
	flush := func() error {
		for _, s := range []byte{1, 2} {
			if b := pending[s]; b.Len() > 0 {
				if err := fn(s, b.Bytes()); err != nil {
					return err
				}
				b.Reset()
			}
		}
		return nil
	}
	for {
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			if ferr := flush(); ferr != nil {
				return ferr
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		stream := hdr[0]
		if stream != 2 {
			stream = 1
		}
		size := binary.BigEndian.Uint32(hdr[4:])
		buf := pending[stream]
		for size > 0 {
			chunk := min(size, 32<<10)
			b := make([]byte, chunk)
			if _, err := io.ReadFull(br, b); err != nil {
				flush()
				return nil
			}
			size -= chunk
			for len(b) > 0 {
				i := bytes.IndexByte(b, '\n')
				if i < 0 {
					if buf.Len() < maxLine+1024 {
						buf.Write(b)
					}
					break
				}
				buf.Write(b[:i])
				if err := fn(stream, buf.Bytes()); err != nil {
					return err
				}
				buf.Reset()
				b = b[i+1:]
			}
		}
	}
}

func ignoreClosed(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// Event is one entry of Docker's event stream.
type Event struct {
	Type   string            `json:"type"`
	Action string            `json:"action"`
	ID     string            `json:"id"`
	Attrs  map[string]string `json:"attrs"`
	Time   int64             `json:"time"` // unix seconds
}

// Events streams daemon events from since (unix seconds, 0 for now) until ctx ends.
func (c *Client) Events(ctx context.Context, since int64, fn func(Event)) error {
	q := filters(map[string][]string{"type": {"container", "image", "volume", "network"}})
	if since > 0 {
		q.Set("since", itoa(since))
	}
	resp, err := c.do(ctx, http.MethodGet, "/events", q, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var e struct {
			Type   string
			Action string
			Actor  struct {
				ID         string
				Attributes map[string]string
			}
			Time int64 `json:"time"`
		}
		if err := dec.Decode(&e); err != nil {
			return ignoreClosed(ctx, err)
		}
		fn(Event{Type: e.Type, Action: e.Action, ID: e.Actor.ID, Attrs: e.Actor.Attributes, Time: e.Time})
	}
}
