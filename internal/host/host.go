// Package host reads the VPS's own vitals from /proc and statfs: CPU time,
// memory, network and disk throughput, load, uptime and disk space.
package host

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Sample is one reading of the host. Rates are per second since the previous Sample.
type Sample struct {
	At        int64   `json:"at"` // unix seconds
	CPUPct    float64 `json:"cpu"`
	Load1     float64 `json:"load1"`
	CPUs      int     `json:"cpus"`
	MemUsed   int64   `json:"mem_used"`
	MemTotal  int64   `json:"mem_total"`
	SwapUsed  int64   `json:"swap_used"`
	SwapTotal int64   `json:"swap_total"`
	DiskUsed  int64   `json:"disk_used"`
	DiskTotal int64   `json:"disk_total"`
	NetRx     float64 `json:"net_rx"`
	NetTx     float64 `json:"net_tx"`
	DiskRead  float64 `json:"disk_read"`
	DiskWrite float64 `json:"disk_write"`
}

// Info is the slow-changing part of the host, read on request.
type Info struct {
	Hostname  string     `json:"hostname"`
	Kernel    string     `json:"kernel"`
	OS        string     `json:"os"`
	CPUs      int        `json:"cpus"`
	CPUModel  string     `json:"cpu_model"`
	UptimeSec int64      `json:"uptime_sec"`
	Load      [3]float64 `json:"load"`
	Disks     []Disk     `json:"disks"`
}

type Disk struct {
	Mount string `json:"mount"`
	Total int64  `json:"total"`
	Used  int64  `json:"used"`
	Free  int64  `json:"free"` // available to unprivileged users, as df's "Avail"
}

// Sampler turns /proc's running counters into rates. Not safe for concurrent use.
type Sampler struct {
	Proc   string   // "/proc"
	Mounts []string // disk space is read for these; the first is the headline disk

	prevAt   time.Time
	prevCPU  cpuTimes
	prevNet  counters
	prevDisk counters
}

type cpuTimes struct{ idle, total uint64 }
type counters struct{ a, b uint64 }

func (s *Sampler) proc(name string) string {
	p := s.Proc
	if p == "" {
		p = "/proc"
	}
	return filepath.Join(p, name)
}

func (s *Sampler) Sample(now time.Time) Sample {
	out := Sample{At: now.Unix()}
	cpu, n := readCPU(s.proc("stat"))
	out.CPUs = n
	m := readMeminfo(s.proc("meminfo"))
	out.MemTotal = m["MemTotal"]
	out.MemUsed = m["MemTotal"] - m["MemAvailable"]
	out.SwapTotal = m["SwapTotal"]
	out.SwapUsed = m["SwapTotal"] - m["SwapFree"]
	out.Load1 = readLoad(s.proc("loadavg"))[0]
	if disks := Disks(s.Mounts); len(disks) > 0 {
		out.DiskUsed, out.DiskTotal = disks[0].Used, disks[0].Total
	}
	net := readNet(s.proc("net/dev"))
	disk := readDiskstats(s.proc("diskstats"))
	if !s.prevAt.IsZero() {
		dt := now.Sub(s.prevAt).Seconds()
		if dt > 0 {
			if total := cpu.total - s.prevCPU.total; total > 0 && cpu.total >= s.prevCPU.total {
				busy := total - (cpu.idle - s.prevCPU.idle)
				out.CPUPct = float64(busy) / float64(total) * 100
			}
			out.NetRx, out.NetTx = rate(net.a, s.prevNet.a, dt), rate(net.b, s.prevNet.b, dt)
			out.DiskRead, out.DiskWrite = rate(disk.a, s.prevDisk.a, dt), rate(disk.b, s.prevDisk.b, dt)
		}
	}
	s.prevAt, s.prevCPU, s.prevNet, s.prevDisk = now, cpu, net, disk
	return out
}

// rate is zero when a counter went backwards (an interface reset or wrap).
func rate(cur, prev uint64, dt float64) float64 {
	if cur < prev {
		return 0
	}
	return float64(cur-prev) / dt
}

func (s *Sampler) Info() Info {
	in := Info{Disks: Disks(s.Mounts), Load: readLoad(s.proc("loadavg"))}
	in.Hostname, _ = os.Hostname()
	if b, err := os.ReadFile(s.proc("sys/kernel/osrelease")); err == nil {
		in.Kernel = strings.TrimSpace(string(b))
	}
	in.OS = osName("/etc/os-release")
	_, in.CPUs = readCPU(s.proc("stat"))
	in.CPUModel = cpuModel(s.proc("cpuinfo"))
	in.UptimeSec = int64(readUptime(s.proc("uptime")))
	return in
}

func readCPU(path string) (cpuTimes, int) {
	f, err := os.Open(path)
	if err != nil {
		return cpuTimes{}, 0
	}
	defer f.Close()
	var t cpuTimes
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if fields[0] != "cpu" {
			n++
			continue
		}
		// user nice system idle iowait irq softirq steal; guest time is already inside user.
		for i, v := range fields[1:] {
			if i >= 8 {
				break
			}
			x, _ := strconv.ParseUint(v, 10, 64)
			t.total += x
			if i == 3 || i == 4 {
				t.idle += x
			}
		}
	}
	return t, n
}

// readMeminfo returns /proc/meminfo in bytes.
func readMeminfo(path string) map[string]int64 {
	out := map[string]int64{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(v)
		if len(fields) == 0 {
			continue
		}
		n, _ := strconv.ParseInt(fields[0], 10, 64)
		if len(fields) > 1 && fields[1] == "kB" {
			n *= 1024
		}
		out[k] = n
	}
	return out
}

func readLoad(path string) [3]float64 {
	var l [3]float64
	b, err := os.ReadFile(path)
	if err != nil {
		return l
	}
	f := strings.Fields(string(b))
	for i := 0; i < 3 && i < len(f); i++ {
		l[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return l
}

func readUptime(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return v
}

// virtualIface matches interfaces whose traffic is already counted on the uplink.
var virtualIface = regexp.MustCompile(`^(lo|veth|docker|br-|virbr|cni|flannel|tailscale|wg)`)

// readNet sums received and sent bytes over the physical interfaces.
func readNet(path string) counters {
	var c counters
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if virtualIface.MatchString(name) {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		c.a += rx
		c.b += tx
	}
	return c
}

// wholeDisk matches block devices, not their partitions, loops or ramdisks.
var wholeDisk = regexp.MustCompile(`^(sd[a-z]+|vd[a-z]+|xvd[a-z]+|hd[a-z]+|nvme\d+n\d+|mmcblk\d+)$`)

// readDiskstats sums bytes read and written over whole disks (sectors are 512 bytes).
func readDiskstats(path string) counters {
	var c counters
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 || !wholeDisk.MatchString(fields[2]) {
			continue
		}
		rd, _ := strconv.ParseUint(fields[5], 10, 64)
		wr, _ := strconv.ParseUint(fields[9], 10, 64)
		c.a += rd * 512
		c.b += wr * 512
	}
	return c
}

// Disks reads space on each mount, skipping paths that live on a filesystem already listed.
func Disks(mounts []string) []Disk {
	out := []Disk{}
	seen := map[[2]int32]bool{}
	for _, m := range mounts {
		var st syscall.Statfs_t
		if err := syscall.Statfs(m, &st); err != nil {
			continue
		}
		if seen[st.Fsid.X__val] {
			continue
		}
		seen[st.Fsid.X__val] = true
		total := int64(st.Blocks) * st.Bsize
		out = append(out, Disk{Mount: m, Total: total, Used: total - int64(st.Bfree)*st.Bsize, Free: int64(st.Bavail) * st.Bsize})
	}
	return out
}

func osName(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func cpuModel(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
