// Package store keeps StunSails' history in SQLite: one-minute host and
// container readings, the activity feed (Docker events and sign-ins/actions),
// sessions, and the compose stacks seen so far.
package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/host"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS host_samples (
	at INTEGER PRIMARY KEY, cpu REAL, load1 REAL, cpus INTEGER,
	mem_used INTEGER, mem_total INTEGER, swap_used INTEGER, swap_total INTEGER,
	disk_used INTEGER, disk_total INTEGER, net_rx REAL, net_tx REAL, disk_read REAL, disk_write REAL
);
CREATE TABLE IF NOT EXISTS ctr_samples (
	id TEXT NOT NULL, at INTEGER NOT NULL, name TEXT, cpu REAL, mem INTEGER,
	PRIMARY KEY (id, at)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS ctr_samples_at ON ctr_samples(at);
CREATE TABLE IF NOT EXISTS activity (
	id INTEGER PRIMARY KEY AUTOINCREMENT, at INTEGER NOT NULL,
	kind TEXT NOT NULL,   -- docker | audit
	type TEXT NOT NULL,   -- container, image, volume, network | login, action
	action TEXT NOT NULL, target TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '',
	ip TEXT NOT NULL DEFAULT '', ok INTEGER NOT NULL DEFAULT 1, detail TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS activity_at ON activity(at);
CREATE INDEX IF NOT EXISTS activity_target ON activity(target);
CREATE TABLE IF NOT EXISTS sessions (
	hash TEXT PRIMARY KEY, created INTEGER NOT NULL, seen INTEGER NOT NULL,
	stepup INTEGER NOT NULL, ip TEXT NOT NULL, ua TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS stacks (
	project TEXT PRIMARY KEY, working_dir TEXT NOT NULL, config_files TEXT NOT NULL,
	env_files TEXT NOT NULL DEFAULT '', last_seen INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS site_checks (
	site TEXT NOT NULL, at INTEGER NOT NULL, up INTEGER NOT NULL, status INTEGER NOT NULL,
	ms INTEGER NOT NULL, cert_expiry INTEGER NOT NULL, error TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (site, at)
) WITHOUT ROWID;
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-2000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Path is the database file, for reporting its size.
func (s *Store) Path() string {
	var file string
	s.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&file)
	return file
}

func (s *Store) Meta(k string) string {
	var v string
	s.db.QueryRow(`SELECT v FROM meta WHERE k = ?`, k).Scan(&v)
	return v
}

func (s *Store) SetMeta(k, v string) {
	s.db.Exec(`INSERT INTO meta (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`, k, v)
}

func (s *Store) AddHost(x host.Sample) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO host_samples VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		x.At, x.CPUPct, x.Load1, x.CPUs, x.MemUsed, x.MemTotal, x.SwapUsed, x.SwapTotal,
		x.DiskUsed, x.DiskTotal, x.NetRx, x.NetTx, x.DiskRead, x.DiskWrite)
	return err
}

// HostHistory averages readings since from into buckets of bucket seconds.
func (s *Store) HostHistory(from, bucket int64) ([]host.Sample, error) {
	rows, err := s.db.Query(`SELECT (at / ?1) * ?1 AS b, AVG(cpu), AVG(load1), MAX(cpus),
		CAST(AVG(mem_used) AS INTEGER), MAX(mem_total), CAST(AVG(swap_used) AS INTEGER), MAX(swap_total),
		MAX(disk_used), MAX(disk_total), AVG(net_rx), AVG(net_tx), AVG(disk_read), AVG(disk_write)
		FROM host_samples WHERE at >= ?2 GROUP BY b ORDER BY b`, bucket, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []host.Sample{}
	for rows.Next() {
		var x host.Sample
		if err := rows.Scan(&x.At, &x.CPUPct, &x.Load1, &x.CPUs, &x.MemUsed, &x.MemTotal, &x.SwapUsed, &x.SwapTotal,
			&x.DiskUsed, &x.DiskTotal, &x.NetRx, &x.NetTx, &x.DiskRead, &x.DiskWrite); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

type CtrSample struct {
	At  int64   `json:"at"`
	CPU float64 `json:"cpu"`
	Mem int64   `json:"mem"`
}

func (s *Store) AddCtr(at int64, id, name string, cpu float64, mem int64) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO ctr_samples VALUES (?,?,?,?,?)`, id, at, name, cpu, mem)
	return err
}

func (s *Store) CtrHistory(id string, from, bucket int64) ([]CtrSample, error) {
	rows, err := s.db.Query(`SELECT (at / ?1) * ?1 AS b, AVG(cpu), CAST(AVG(mem) AS INTEGER)
		FROM ctr_samples WHERE id = ?2 AND at >= ?3 GROUP BY b ORDER BY b`, bucket, id, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CtrSample{}
	for rows.Next() {
		var x CtrSample
		if err := rows.Scan(&x.At, &x.CPU, &x.Mem); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// migrate adds columns that came after the first schema.
func migrate(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('activity') WHERE name = 'data'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE activity ADD COLUMN data TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// Source is one address behind a summed-up line, such as failed SSH logins.
type Source struct {
	IP       string   `json:"ip"`
	Attempts int      `json:"attempts"`
	Users    []string `json:"users,omitempty"`
}

// Activity is one line of the feed.
type Activity struct {
	ID     int64  `json:"id"`
	At     int64  `json:"at"`
	Kind   string `json:"kind"`
	Type   string `json:"type"`
	Action string `json:"action"`
	Target string `json:"target"`
	Name   string `json:"name"`
	IP     string `json:"ip,omitempty"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	// Sources is filled only by ActivityByID; lists leave it out.
	Sources []Source `json:"sources,omitempty"`
}

func (s *Store) AddActivity(a Activity) error {
	if a.At == 0 {
		a.At = time.Now().Unix()
	}
	data := ""
	if len(a.Sources) > 0 {
		b, err := json.Marshal(a.Sources)
		if err != nil {
			return err
		}
		data = string(b)
	}
	_, err := s.db.Exec(`INSERT INTO activity (at, kind, type, action, target, name, ip, ok, detail, data) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		a.At, a.Kind, a.Type, a.Action, a.Target, a.Name, a.IP, a.OK, a.Detail, data)
	return err
}

type ActivityQuery struct {
	Kind   string // docker, audit, or "" for both
	Type   string
	Target string // container id or name
	Q      string // matches name, action or detail
	Before int64  // id cursor; 0 for newest
	Limit  int
}

// filter turns a query into a WHERE clause and its arguments.
func (q ActivityQuery) filter() (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if q.Kind != "" {
		where, args = append(where, "kind = ?"), append(args, q.Kind)
	}
	if q.Type != "" {
		where, args = append(where, "type = ?"), append(args, q.Type)
	}
	if q.Target != "" {
		where, args = append(where, "(target = ? OR name = ?)"), append(args, q.Target, q.Target)
	}
	if q.Q != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q.Q) + "%"
		where = append(where, `(name LIKE ? ESCAPE '\' OR action LIKE ? ESCAPE '\' OR detail LIKE ? ESCAPE '\' OR ip LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like)
	}
	if q.Before > 0 {
		where, args = append(where, "id < ?"), append(args, q.Before)
	}
	return strings.Join(where, " AND "), args
}

const activityCols = `id, at, kind, type, action, target, name, ip, ok, detail`

func scanActivity(rows *sql.Rows) ([]Activity, error) {
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.At, &a.Kind, &a.Type, &a.Action, &a.Target, &a.Name, &a.IP, &a.OK, &a.Detail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) Activity(q ActivityQuery) ([]Activity, error) {
	where, args := q.filter()
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 100
	}
	rows, err := s.db.Query(`SELECT `+activityCols+` FROM activity WHERE `+where+` ORDER BY id DESC LIMIT ?`, append(args, q.Limit)...)
	if err != nil {
		return nil, err
	}
	return scanActivity(rows)
}

// ActivityPage is one page of the feed by offset, with how many lines match in all.
// Oldest first when asc; newest first otherwise.
func (s *Store) ActivityPage(q ActivityQuery, offset int, asc bool) ([]Activity, int, error) {
	where, args := q.filter()
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM activity WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 100
	}
	order := "DESC"
	if asc {
		order = "ASC"
	}
	rows, err := s.db.Query(`SELECT `+activityCols+` FROM activity WHERE `+where+` ORDER BY at `+order+`, id `+order+` LIMIT ? OFFSET ?`,
		append(args, q.Limit, max(offset, 0))...)
	if err != nil {
		return nil, 0, err
	}
	items, err := scanActivity(rows)
	return items, total, err
}

// ActivityByID is one line with its sources; ok is false when it's gone.
func (s *Store) ActivityByID(id int64) (Activity, bool, error) {
	var a Activity
	var data string
	err := s.db.QueryRow(`SELECT `+activityCols+`, data FROM activity WHERE id = ?`, id).
		Scan(&a.ID, &a.At, &a.Kind, &a.Type, &a.Action, &a.Target, &a.Name, &a.IP, &a.OK, &a.Detail, &data)
	if err == sql.ErrNoRows {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	if data != "" {
		json.Unmarshal([]byte(data), &a.Sources)
	}
	return a, true, nil
}

type Session struct {
	Hash    string
	Created time.Time
	Seen    time.Time
	StepUp  time.Time
	IP, UA  string
}

func (s *Store) CreateSession(x Session) error {
	_, err := s.db.Exec(`INSERT INTO sessions VALUES (?,?,?,?,?,?)`, x.Hash, x.Created.Unix(), x.Seen.Unix(), x.StepUp.Unix(), x.IP, x.UA)
	return err
}

func (s *Store) Session(hash string) (Session, bool) {
	var x Session
	var c, seen, st int64
	err := s.db.QueryRow(`SELECT hash, created, seen, stepup, ip, ua FROM sessions WHERE hash = ?`, hash).Scan(&x.Hash, &c, &seen, &st, &x.IP, &x.UA)
	if err != nil {
		return x, false
	}
	x.Created, x.Seen, x.StepUp = time.Unix(c, 0), time.Unix(seen, 0), time.Unix(st, 0)
	return x, true
}

func (s *Store) TouchSession(hash string, now time.Time) {
	s.db.Exec(`UPDATE sessions SET seen = ? WHERE hash = ?`, now.Unix(), hash)
}

func (s *Store) StepUpSession(hash string, now time.Time) {
	s.db.Exec(`UPDATE sessions SET stepup = ?, seen = ? WHERE hash = ?`, now.Unix(), now.Unix(), hash)
}

func (s *Store) DeleteSession(hash string) {
	s.db.Exec(`DELETE FROM sessions WHERE hash = ?`, hash)
}

// DeleteOtherSessions signs out everywhere but keep.
func (s *Store) DeleteOtherSessions(keep string) (int64, error) {
	r, err := s.db.Exec(`DELETE FROM sessions WHERE hash != ?`, keep)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

type SessionInfo struct {
	ID      string `json:"id"` // first 8 hex of the hash, to tell sessions apart
	Created int64  `json:"created"`
	Seen    int64  `json:"seen"`
	IP      string `json:"ip"`
	UA      string `json:"ua"`
	Current bool   `json:"current"`
}

func (s *Store) Sessions(current string) ([]SessionInfo, error) {
	rows, err := s.db.Query(`SELECT hash, created, seen, ip, ua FROM sessions ORDER BY seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionInfo{}
	for rows.Next() {
		var h string
		var x SessionInfo
		if err := rows.Scan(&h, &x.Created, &x.Seen, &x.IP, &x.UA); err != nil {
			return nil, err
		}
		x.ID, x.Current = h[:8], h == current
		out = append(out, x)
	}
	return out, rows.Err()
}

// StackRow remembers where a compose project lives, so it can be brought back
// up after `down` removed every container that carried the labels.
type StackRow struct {
	Project     string
	WorkingDir  string
	ConfigFiles []string
	EnvFiles    []string
	LastSeen    int64
}

func (s *Store) UpsertStack(x StackRow) error {
	_, err := s.db.Exec(`INSERT INTO stacks VALUES (?,?,?,?,?) ON CONFLICT(project) DO UPDATE SET
		working_dir = excluded.working_dir, config_files = excluded.config_files,
		env_files = excluded.env_files, last_seen = excluded.last_seen`,
		x.Project, x.WorkingDir, strings.Join(x.ConfigFiles, "\n"), strings.Join(x.EnvFiles, "\n"), x.LastSeen)
	return err
}

func (s *Store) Stacks() ([]StackRow, error) {
	rows, err := s.db.Query(`SELECT project, working_dir, config_files, env_files, last_seen FROM stacks ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StackRow{}
	for rows.Next() {
		var x StackRow
		var cf, ef string
		if err := rows.Scan(&x.Project, &x.WorkingDir, &cf, &ef, &x.LastSeen); err != nil {
			return nil, err
		}
		x.ConfigFiles, x.EnvFiles = split(cf), split(ef)
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) ForgetStack(project string) error {
	_, err := s.db.Exec(`DELETE FROM stacks WHERE project = ?`, project)
	return err
}

func split(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// Retention: readings for 7 days, the activity feed for 30, sessions by their own limits.
func (s *Store) Prune(now time.Time, idle, maxAge time.Duration) {
	s.db.Exec(`DELETE FROM host_samples WHERE at < ?`, now.Add(-7*24*time.Hour).Unix())
	s.db.Exec(`DELETE FROM ctr_samples WHERE at < ?`, now.Add(-7*24*time.Hour).Unix())
	s.db.Exec(`DELETE FROM activity WHERE at < ?`, now.Add(-30*24*time.Hour).Unix())
	s.db.Exec(`DELETE FROM site_checks WHERE at < ?`, now.Add(-7*24*time.Hour).Unix())
	s.db.Exec(`DELETE FROM sessions WHERE seen < ? OR created < ?`, now.Add(-idle).Unix(), now.Add(-maxAge).Unix())
}

// DiskForecast fits a line through disk use since from and returns the days
// until the disk is full, or -1 when use isn't growing or there is under a day
// of readings to go on.
func (s *Store) DiskForecast(from int64) (float64, error) {
	var n, sx, sy, sxy, sxx, minAt, maxAt, total, last sql.NullFloat64
	err := s.db.QueryRow(`SELECT COUNT(*), SUM(at), SUM(disk_used), SUM(at * disk_used), SUM(at * at), MIN(at), MAX(at),
		(SELECT disk_total FROM host_samples ORDER BY at DESC LIMIT 1), (SELECT disk_used FROM host_samples ORDER BY at DESC LIMIT 1)
		FROM (SELECT at - ?1 AS at, CAST(disk_used AS REAL) AS disk_used FROM host_samples WHERE at >= ?1)`, from).
		Scan(&n, &sx, &sy, &sxy, &sxx, &minAt, &maxAt, &total, &last)
	if err != nil {
		return -1, err
	}
	return forecast(n.Float64, sx.Float64, sy.Float64, sxy.Float64, sxx.Float64, maxAt.Float64-minAt.Float64, total.Float64, last.Float64), nil
}

func forecast(n, sx, sy, sxy, sxx, span, total, last float64) float64 {
	if n < 30 || span < 24*3600 || total <= 0 {
		return -1
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return -1
	}
	slope := (n*sxy - sx*sy) / den // bytes per second
	if slope <= 0 {
		return -1
	}
	return (total - last) / slope / 86400
}

// SiteCheck is one probe of a site.
type SiteCheck struct {
	At         int64  `json:"at"`
	Up         bool   `json:"up"`
	Status     int    `json:"status"`
	Ms         int64  `json:"ms"`
	CertExpiry int64  `json:"cert_expiry"`
	Error      string `json:"error,omitempty"`
}

func (s *Store) AddSiteCheck(site string, c SiteCheck) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO site_checks VALUES (?,?,?,?,?,?,?)`, site, c.At, c.Up, c.Status, c.Ms, c.CertExpiry, c.Error)
	return err
}

// SiteHistory averages checks since from into buckets; Up is true only when every check in the bucket was up.
func (s *Store) SiteHistory(site string, from, bucket int64) ([]SiteCheck, error) {
	rows, err := s.db.Query(`SELECT (at / ?1) * ?1 AS b, MIN(up), MAX(status), CAST(AVG(ms) AS INTEGER), MAX(cert_expiry), MAX(error)
		FROM site_checks WHERE site = ?2 AND at >= ?3 GROUP BY b ORDER BY b`, bucket, site, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SiteCheck{}
	for rows.Next() {
		var c SiteCheck
		if err := rows.Scan(&c.At, &c.Up, &c.Status, &c.Ms, &c.CertExpiry, &c.Error); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SiteUptime is the share of checks that were up since from, or -1 without checks.
func (s *Store) SiteUptime(site string, from int64) float64 {
	var n, up sql.NullFloat64
	s.db.QueryRow(`SELECT COUNT(*), SUM(up) FROM site_checks WHERE site = ? AND at >= ?`, site, from).Scan(&n, &up)
	if n.Float64 == 0 {
		return -1
	}
	return up.Float64 / n.Float64 * 100
}
