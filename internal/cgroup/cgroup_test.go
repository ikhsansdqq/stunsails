package cgroup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "system.slice", "docker-abc.scope")
	os.MkdirAll(dir, 0o755)
	files := map[string]string{
		"cpu.stat":       "usage_usec 123456\nuser_usec 100000\n",
		"memory.current": "1048576\n",
		"memory.stat":    "anon 500000\ninactive_file 48576\n",
		"memory.max":     "max\n",
		"pids.current":   "7\n",
		"io.stat":        "8:0 rbytes=100 wbytes=200 rios=1 wios=2\n\n8:16 rbytes=1 wbytes=2\n",
	}
	for k, v := range files {
		os.WriteFile(filepath.Join(dir, k), []byte(v), 0o644)
	}
	r, err := Read(root, "abc")
	if err != nil {
		t.Fatal(err)
	}
	want := Reading{CPUUsec: 123456, Mem: 1000000, MemLimit: 0, IORead: 101, IOWrite: 202, Pids: 7}
	if r != want {
		t.Fatalf("got %+v, want %+v", r, want)
	}
	if _, err := Read(root, "missing"); err == nil {
		t.Fatal("want error for a container without a cgroup")
	}
}
