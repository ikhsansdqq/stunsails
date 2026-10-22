package docker

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func frame(stream byte, s string) []byte {
	h := make([]byte, 8)
	h[0] = stream
	binary.BigEndian.PutUint32(h[4:], uint32(len(s)))
	return append(h, s...)
}

func TestDemux(t *testing.T) {
	var in bytes.Buffer
	in.Write(frame(1, "hello\nwor"))
	in.Write(frame(2, "oops\n"))
	in.Write(frame(1, "ld\n"))
	in.Write(frame(1, "no newline"))
	var got []string
	err := demux(&in, func(s byte, l []byte) error {
		got = append(got, string(rune('0'+s))+":"+string(l))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1:hello", "2:oops", "1:world", "1:no newline"}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestHealth(t *testing.T) {
	for in, want := range map[string]string{
		"Up 3 hours (healthy)":           "healthy",
		"Up 1 minute (health: starting)": "starting",
		"Up 2 days (unhealthy)":          "unhealthy",
		"Exited (0) 3 days ago":          "",
	} {
		if got := health(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
