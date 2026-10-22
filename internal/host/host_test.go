package host

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSamplerRates(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "meminfo", "MemTotal: 1000 kB\nMemAvailable: 250 kB\nSwapTotal: 100 kB\nSwapFree: 40 kB\n")
	write(t, dir, "loadavg", "0.50 0.40 0.30 1/100 1234\n")
	write(t, dir, "stat", "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 50 0 50 400 0 0 0 0\ncpu1 50 0 50 400 0 0 0 0\nintr 1\n")
	write(t, dir, "net/dev", "Inter-|\n face |bytes\n    lo: 999 0 0 0 0 0 0 0 999 0 0 0 0 0 0 0\n  eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0\nveth1: 5 0 0 0 0 0 0 0 5 0 0 0 0 0 0 0\n")
	write(t, dir, "diskstats", "   8       0 vda 10 0 100 0 20 0 200 0 0 0 0\n   8       1 vda1 10 0 100 0 20 0 200 0 0 0 0\n   7 0 loop0 1 0 9999 0 1 0 9999 0 0 0 0\n")
	s := &Sampler{Proc: dir}
	t0 := time.Unix(1000, 0)
	first := s.Sample(t0)
	if first.CPUs != 2 || first.MemUsed != 750*1024 || first.SwapUsed != 60*1024 || first.Load1 != 0.5 {
		t.Fatalf("first sample wrong: %+v", first)
	}
	if first.CPUPct != 0 {
		t.Fatal("first sample has no rate")
	}
	// 10 s later: 100 more busy jiffies out of 200 total, eth0 +1000/+500 bytes, vda +20/+40 sectors.
	write(t, dir, "stat", "cpu  150 0 150 900 0 0 0 0 0 0\ncpu0 0\ncpu1 0\n")
	write(t, dir, "net/dev", "  eth0: 2000 0 0 0 0 0 0 0 2500 0 0 0 0 0 0 0\n    lo: 5000 0 0 0 0 0 0 0 5000 0 0 0 0 0 0 0\n")
	write(t, dir, "diskstats", "   8       0 vda 10 0 120 0 20 0 240 0 0 0 0\n")
	x := s.Sample(t0.Add(10 * time.Second))
	if x.CPUPct != 50 {
		t.Errorf("cpu %v, want 50", x.CPUPct)
	}
	if x.NetRx != 100 || x.NetTx != 50 {
		t.Errorf("net %v/%v, want 100/50", x.NetRx, x.NetTx)
	}
	if x.DiskRead != 20*512/10 || x.DiskWrite != 40*512/10 {
		t.Errorf("disk %v/%v", x.DiskRead, x.DiskWrite)
	}
	// A counter going backwards (interface reset) reads as zero, not a huge number.
	write(t, dir, "net/dev", "  eth0: 10 0 0 0 0 0 0 0 10 0 0 0 0 0 0 0\n")
	if y := s.Sample(t0.Add(20 * time.Second)); y.NetRx != 0 || y.NetTx != 0 {
		t.Errorf("reset counters should read 0, got %v/%v", y.NetRx, y.NetTx)
	}
}

func TestDisksDedup(t *testing.T) {
	d := Disks([]string{"/", "/", "/does-not-exist"})
	if len(d) != 1 || d[0].Total <= 0 || d[0].Used <= 0 || d[0].Free <= 0 {
		t.Fatalf("disks %+v", d)
	}
}
