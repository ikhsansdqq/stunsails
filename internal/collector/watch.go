package collector

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/stunsails/stunsails/internal/alert"
	"github.com/stunsails/stunsails/internal/docker"
	"github.com/stunsails/stunsails/internal/host"
)

// LoopWindow and LoopDies: a container that died this many times within the
// window is in a restart loop.
const (
	LoopWindow = 15 * time.Minute
	LoopDies   = 3
)

// health is what the alert rules remember between ticks.
type health struct {
	mu         sync.Mutex
	dies       map[string][]time.Time // container id -> recent die times
	looping    map[string]string      // id -> name, currently flagged
	unhealthy  map[string]bool        // names currently unhealthy
	memHigh    int                    // consecutive ticks with memory over 90%
	forecast   float64                // days until the disk is full, -1 unknown
	forecastAt time.Time
}

func (h *health) init() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dies, h.looping, h.unhealthy, h.forecast = map[string][]time.Time{}, map[string]string{}, map[string]bool{}, -1
}

// Looping reports the ids of containers in a restart loop.
func (c *Collector) Looping() map[string]bool {
	c.health.mu.Lock()
	defer c.health.mu.Unlock()
	out := map[string]bool{}
	for id := range c.health.looping {
		out[id] = true
	}
	return out
}

// Forecast is the days until the disk fills at the last week's pace, or -1.
func (c *Collector) Forecast() float64 {
	c.health.mu.Lock()
	defer c.health.mu.Unlock()
	return c.health.forecast
}

// recentDies drops die times older than the window and returns how many remain.
func recentDies(ts []time.Time, now time.Time) []time.Time {
	return slices.DeleteFunc(ts, func(t time.Time) bool { return now.Sub(t) > LoopWindow })
}

// onEvent feeds Docker events to the rules that react to them at once.
func (c *Collector) onEvent(e docker.Event) {
	if e.Type != "container" {
		return
	}
	name := e.Attrs["name"]
	switch e.Action {
	case "die":
		c.health.mu.Lock()
		if c.health.dies != nil {
			c.health.dies[e.ID] = append(recentDies(c.health.dies[e.ID], time.Now()), time.Unix(e.Time, 0))
		}
		c.health.mu.Unlock()
		code := e.Attrs["exitCode"]
		if c.Alerts != nil && code != "" && code != "0" && code != "137" && code != "143" {
			c.Alerts.Once(alert.RuleCrash, name, name+" crashed", fmt.Sprintf("It exited with code %s (image %s).", code, e.Attrs["image"]), "critical")
		}
	case "oom":
		if c.Alerts != nil {
			c.Alerts.Once(alert.RuleOOM, name, name+" ran out of memory", "The kernel killed a process in it. Raise its memory limit or find the leak.", "critical")
		}
	}
}

// evaluate runs the rules that look at readings: loops, health, memory, disk.
func (c *Collector) evaluate(now time.Time, hs host.Sample, ctrs []docker.Container) {
	h := &c.health
	h.mu.Lock()
	if h.dies == nil {
		h.mu.Unlock()
		return
	}
	// Restart loops: too many deaths lately, or Docker itself says restarting.
	looping := map[string]string{}
	for _, ct := range ctrs {
		d := recentDies(h.dies[ct.ID], now)
		h.dies[ct.ID] = d
		if len(d) >= LoopDies || ct.State == "restarting" {
			looping[ct.ID] = ct.Name
		}
	}
	for id, d := range h.dies {
		if len(d) == 0 {
			delete(h.dies, id)
		}
	}
	prevLoop := h.looping
	h.looping = looping
	unhealthy := map[string]bool{}
	for _, ct := range ctrs {
		if ct.Health == "unhealthy" {
			unhealthy[ct.Name] = true
		}
	}
	prevUnhealthy := h.unhealthy
	h.unhealthy = unhealthy
	memPct := 0.0
	if hs.MemTotal > 0 {
		memPct = float64(hs.MemUsed) / float64(hs.MemTotal) * 100
	}
	if memPct > 90 {
		h.memHigh++
	} else if memPct < 85 {
		h.memHigh = 0
	}
	memHigh := h.memHigh
	refresh := now.Sub(h.forecastAt) > time.Hour
	h.mu.Unlock()

	if refresh {
		days, err := c.Store.DiskForecast(now.Add(-7 * 24 * time.Hour).Unix())
		if err != nil {
			days = -1
		}
		h.mu.Lock()
		h.forecast, h.forecastAt = days, now
		h.mu.Unlock()
		if c.Alerts != nil {
			if days >= 0 && days < 7 {
				c.Alerts.Fire(alert.RuleDisk, "forecast", fmt.Sprintf("Disk full in about %.0f days", days), "At last week's pace the disk fills within a week. Clean up in StunSails' Storage page.", "warning")
			} else {
				c.Alerts.Resolve(alert.RuleDisk, "forecast", "Disk growth slowed down")
			}
		}
	}
	if c.Alerts == nil {
		return
	}
	for id, name := range looping {
		if _, was := prevLoop[id]; !was {
			c.Alerts.Fire(alert.RuleLoop, name, name+" keeps restarting", fmt.Sprintf("It died %d or more times in %d minutes. Check its logs.", LoopDies, int(LoopWindow.Minutes())), "critical")
		}
	}
	for id, name := range prevLoop {
		if _, still := looping[id]; !still {
			c.Alerts.Resolve(alert.RuleLoop, name, name+" stopped restarting")
		}
	}
	for name := range unhealthy {
		c.Alerts.Fire(alert.RuleUnhealthy, name, name+" is unhealthy", "Its health check is failing.", "critical")
	}
	for name := range prevUnhealthy {
		if !unhealthy[name] {
			c.Alerts.Resolve(alert.RuleUnhealthy, name, name+" is healthy again")
		}
	}
	if memHigh >= 30 {
		c.Alerts.Fire(alert.RuleMemory, "memory", fmt.Sprintf("Memory at %.0f%%", memPct), "Over 90% for five minutes. See the busiest containers in StunSails.", "warning")
	} else if memHigh == 0 {
		c.Alerts.Resolve(alert.RuleMemory, "memory", "Memory is back under 85%")
	}
	if hs.DiskTotal > 0 {
		diskPct := float64(hs.DiskUsed) / float64(hs.DiskTotal) * 100
		if diskPct > 90 {
			c.Alerts.Fire(alert.RuleDisk, "/", fmt.Sprintf("Disk %.0f%% full", diskPct), "Free space in StunSails' Storage page: build cache and unused images are the usual culprits.", "critical")
		} else if diskPct < 88 {
			c.Alerts.Resolve(alert.RuleDisk, "/", "Disk is back under 88%")
		}
	}
}
