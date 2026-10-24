package alert

import (
	"testing"
	"time"
)

func TestFeedKeepsTheNewest(t *testing.T) {
	n := &Notifier{active: map[string]*state{}}
	for i := 0; i < feedSize+10; i++ {
		n.remember(Message{Rule: "r", Title: "t", At: time.Unix(int64(i), 0)})
	}
	items, last := n.Feed(0)
	if len(items) != feedSize || last != feedSize+10 || items[0].Seq != 11 {
		t.Fatalf("got %d items, last %d, first %d", len(items), last, items[0].Seq)
	}
	if items, _ := n.Feed(last - 2); len(items) != 2 || items[1].Seq != last {
		t.Fatalf("after: %+v", items)
	}
}
