package collector

import (
	"testing"
	"time"

	"server-monitor/internal/docker"
	"server-monitor/internal/host"
)

func TestLoopDetection(t *testing.T) {
	c := &Collector{}
	c.health.init()
	now := time.Now()
	for i := range 3 {
		c.onEvent(docker.Event{Type: "container", Action: "die", ID: "a", Time: now.Add(time.Duration(-i) * time.Minute).Unix(), Attrs: map[string]string{"name": "web", "exitCode": "1"}})
	}
	c.onEvent(docker.Event{Type: "container", Action: "die", ID: "b", Time: now.Unix(), Attrs: map[string]string{"name": "db", "exitCode": "0"}})
	c.health.forecastAt = now // skip the store
	ctrs := []docker.Container{{ID: "a", Name: "web", State: "running"}, {ID: "b", Name: "db", State: "exited"}, {ID: "c", Name: "x", State: "restarting"}}
	c.evaluate(now, host.Sample{}, ctrs)
	l := c.Looping()
	if !l["a"] || l["b"] || !l["c"] {
		t.Fatalf("looping %v", l)
	}
	// Quiet for longer than the window: no longer looping.
	ctrs[2].State = "running"
	c.evaluate(now.Add(LoopWindow+time.Minute), host.Sample{}, ctrs)
	if len(c.Looping()) != 0 {
		t.Fatalf("still looping %v", c.Looping())
	}
}
