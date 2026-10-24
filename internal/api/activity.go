package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ikhsansdqq/stunsails/internal/geo"
	"github.com/ikhsansdqq/stunsails/internal/store"
)

func activityQuery(r *http.Request) store.ActivityQuery {
	q := r.URL.Query()
	kind := q.Get("kind")
	if kind != "docker" && kind != "audit" && kind != "host" && kind != "alert" {
		kind = ""
	}
	return store.ActivityQuery{Kind: kind, Type: q.Get("type"), Target: q.Get("target"), Q: strings.TrimSpace(q.Get("q"))}
}

// activityPage is the Activity table's page: offset paging with the total, so
// the table can show "26–50 of 1,204" and jump between pages.
func (s *Server) activityPage(w http.ResponseWriter, r *http.Request, _ sess) {
	q := activityQuery(r)
	v := r.URL.Query()
	q.Limit, _ = strconv.Atoi(v.Get("limit"))
	if q.Limit != 25 && q.Limit != 50 && q.Limit != 100 {
		q.Limit = 25
	}
	offset, _ := strconv.Atoi(v.Get("offset"))
	items, total, err := s.Store.ActivityPage(q, offset, v.Get("order") == "asc")
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

// Located is an address with where it is.
type Located struct {
	store.Source
	geo.Info
}

// ActivityDetail is one feed line opened up: every address behind it, placed on the map.
type ActivityDetail struct {
	store.Activity
	Sources []Located `json:"sources"`
	// Geo says whether the location database is loaded, so the page can tell
	// "no location known" from "not set up yet".
	Geo bool `json:"geo"`
}

func (s *Server) activityDetail(w http.ResponseWriter, r *http.Request, _ sess) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "bad id")
		return
	}
	a, ok, err := s.Store.ActivityByID(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		fail(w, http.StatusNotFound, "That entry is gone; entries are kept for 30 days.")
		return
	}
	srcs := a.Sources
	if len(srcs) == 0 && a.Kind == "host" && a.Type == "ssh" && a.Action == "failed logins" {
		srcs = legacySources(a.Detail)
	}
	if len(srcs) == 0 && a.IP != "" {
		srcs = []store.Source{{IP: a.IP}}
	}
	out := ActivityDetail{Activity: a, Sources: make([]Located, 0, len(srcs))}
	out.Activity.Sources = nil
	for _, src := range srcs {
		l := Located{Source: src}
		if s.Geo != nil {
			l.Info = s.Geo.Lookup(src.IP)
		}
		out.Sources = append(out.Sources, l)
	}
	out.Geo = s.Geo != nil && s.Geo.Ready()
	writeJSON(w, http.StatusOK, out)
}

var legacyRe = regexp.MustCompile(`([0-9A-Fa-f.:]+) \((\d+) attempts\)`)

// legacySources reads the top addresses out of a summary written before
// summaries kept every address: "1.2.3.4 (40 attempts), … · users tried: …".
func legacySources(detail string) []store.Source {
	var out []store.Source
	for _, m := range legacyRe.FindAllStringSubmatch(detail, -1) {
		n, _ := strconv.Atoi(m[2])
		out = append(out, store.Source{IP: m[1], Attempts: n})
	}
	return out
}
