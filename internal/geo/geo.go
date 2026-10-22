// Package geo places IP addresses on the map with DB-IP's free Lite databases
// (CC BY 4.0, https://db-ip.com), kept on disk so no address leaves the server.
// The city and ASN files are fetched once a month and swapped in while running;
// until the first download finishes, lookups come back empty.
package geo

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// Info is where an address is, as far as DB-IP knows.
type Info struct {
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Lat         float64 `json:"lat,omitempty"`
	Lon         float64 `json:"lon,omitempty"`
	ASN         uint    `json:"asn,omitempty"`
	Org         string  `json:"org,omitempty"`
	Private     bool    `json:"private,omitempty"`
}

type cityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Location struct {
		Lat float64 `maxminddb:"latitude"`
		Lon float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

type asnRecord struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

// DB looks addresses up in the city and ASN files under Dir.
type DB struct {
	Dir string
	// URL is the download address for a kind ("city", "asn") and month ("2026-10").
	URL    func(kind, month string) string
	Client *http.Client
	Now    func() time.Time

	mu      sync.RWMutex
	readers map[string]*maxminddb.Reader
	lastErr string
}

const maxAge = 35 * 24 * time.Hour

var kinds = []string{"city", "asn"}

func dbipURL(kind, month string) string {
	return fmt.Sprintf("https://download.db-ip.com/free/dbip-%s-lite-%s.mmdb.gz", kind, month)
}

func (d *DB) path(kind string) string { return filepath.Join(d.Dir, "dbip-"+kind+"-lite.mmdb") }

func (d *DB) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// Open loads whatever files are already on disk.
func (d *DB) Open() {
	for _, k := range kinds {
		if _, err := os.Stat(d.path(k)); err == nil {
			d.load(k)
		}
	}
}

func (d *DB) load(kind string) error {
	r, err := maxminddb.Open(d.path(kind))
	if err != nil {
		return err
	}
	d.mu.Lock()
	if d.readers == nil {
		d.readers = map[string]*maxminddb.Reader{}
	}
	old := d.readers[kind]
	d.readers[kind] = r
	d.mu.Unlock()
	if old != nil {
		// Lookups copy what they decode, so the old map can go once swapped out.
		old.Close()
	}
	return nil
}

// Ready reports whether the city file is loaded.
func (d *DB) Ready() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.readers["city"] != nil
}

// Err is why the last refresh failed, or "".
func (d *DB) Err() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastErr
}

// Lookup places one address; unknown and unparsable addresses give an empty Info.
func (d *DB) Lookup(s string) Info {
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return Info{}
	}
	ip = ip.Unmap()
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || isCGNAT(ip) {
		return Info{Private: true}
	}
	var out Info
	d.mu.RLock()
	defer d.mu.RUnlock()
	if r := d.readers["city"]; r != nil {
		var c cityRecord
		if r.Lookup(ip).Decode(&c) == nil {
			out.Country, out.CountryCode, out.City = c.Country.Names["en"], c.Country.ISOCode, c.City.Names["en"]
			if len(c.Subdivisions) > 0 {
				out.Region = c.Subdivisions[0].Names["en"]
			}
			out.Lat, out.Lon = c.Location.Lat, c.Location.Lon
		}
	}
	if r := d.readers["asn"]; r != nil {
		var a asnRecord
		if r.Lookup(ip).Decode(&a) == nil {
			out.ASN, out.Org = a.Number, a.Org
		}
	}
	return out
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

func isCGNAT(ip netip.Addr) bool { return cgnat.Contains(ip) }

// Run keeps the files fresh: at start and then daily, anything missing or over
// 35 days old is downloaded again.
func (d *DB) Run(ctx context.Context) {
	d.Open()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		d.Refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Refresh downloads the files that are missing or stale.
func (d *DB) Refresh(ctx context.Context) {
	var errs []string
	for _, k := range kinds {
		if st, err := os.Stat(d.path(k)); err == nil && d.now().Sub(st.ModTime()) < maxAge {
			continue
		}
		if err := d.fetch(ctx, k); err != nil {
			errs = append(errs, k+": "+err.Error())
			log.Printf("geo: %s: %v", k, err)
		}
	}
	d.mu.Lock()
	d.lastErr = ""
	if len(errs) > 0 {
		d.lastErr = fmt.Sprint(errs)
	}
	d.mu.Unlock()
}

// fetch downloads this month's file, or last month's early in the month before
// DB-IP has published, unzips it next to the live one and swaps it in.
func (d *DB) fetch(ctx context.Context, kind string) error {
	if err := os.MkdirAll(d.Dir, 0o700); err != nil {
		return err
	}
	url := d.URL
	if url == nil {
		url = dbipURL
	}
	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	now := d.now().UTC()
	var lastErr error
	for _, month := range []time.Time{now, now.AddDate(0, 0, -now.Day())} {
		lastErr = d.download(ctx, client, url(kind, month.Format("2006-01")), kind)
		if lastErr == nil {
			return d.load(kind)
		}
	}
	return lastErr
}

func (d *DB) download(ctx context.Context, client *http.Client, url, kind string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, res.Status)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d.Dir, "dbip-"+kind+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	// The city file is about 130 MB unzipped; 512 MB stops a runaway download.
	if _, err := io.Copy(tmp, io.LimitReader(zr, 512<<20)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Check it opens before it replaces a working file.
	r, err := maxminddb.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("downloaded file is not a database: %w", err)
	}
	r.Close()
	return os.Rename(tmp.Name(), d.path(kind))
}
