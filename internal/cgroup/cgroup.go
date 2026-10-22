// Package cgroup reads a container's CPU, memory, IO and process counters
// straight from cgroup v2 files. That's far cheaper than Docker's /stats call,
// which waits a second per container to compute a CPU delta.
package cgroup

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Reading struct {
	CPUUsec  uint64 // cumulative CPU time
	Mem      int64  // memory.current minus inactive file cache, as `docker stats` shows
	MemLimit int64  // 0 when unlimited
	IORead   uint64 // cumulative bytes
	IOWrite  uint64
	Pids     int64
}

// Dir finds the cgroup folder of a container under root ("/sys/fs/cgroup"),
// for both the systemd and the cgroupfs cgroup drivers.
func Dir(root, id string) (string, error) {
	for _, p := range []string{
		filepath.Join(root, "system.slice", "docker-"+id+".scope"),
		filepath.Join(root, "docker", id),
	} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("no cgroup for container")
}

func Read(root, id string) (Reading, error) {
	dir, err := Dir(root, id)
	if err != nil {
		return Reading{}, err
	}
	var r Reading
	stat := keyed(filepath.Join(dir, "cpu.stat"))
	r.CPUUsec = uint64(stat["usage_usec"])
	r.Mem = single(filepath.Join(dir, "memory.current"))
	if inactive := keyed(filepath.Join(dir, "memory.stat"))["inactive_file"]; inactive < r.Mem {
		r.Mem -= inactive
	}
	r.MemLimit = single(filepath.Join(dir, "memory.max")) // "max" parses as 0
	r.Pids = single(filepath.Join(dir, "pids.current"))
	r.IORead, r.IOWrite = ioStat(filepath.Join(dir, "io.stat"))
	return r, nil
}

func single(path string) int64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

// keyed reads "name value" lines.
func keyed(path string) map[string]int64 {
	out := map[string]int64{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		out[k] = n
	}
	return out
}

// ioStat sums rbytes and wbytes over every device line ("8:0 rbytes=1 wbytes=2 ...").
func ioStat(path string) (rd, wr uint64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		for _, kv := range fields[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			n, _ := strconv.ParseUint(v, 10, 64)
			switch k {
			case "rbytes":
				rd += n
			case "wbytes":
				wr += n
			}
		}
	}
	return rd, wr
}
