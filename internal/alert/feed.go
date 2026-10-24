package alert

// feedSize is how many alerts the feed keeps. An app reading it every few
// seconds never falls that far behind; one that does sees a gap in Seq.
const feedSize = 256

// FeedItem is one alert as the feed hands it out: Seq rises by one per alert
// for as long as the process runs.
type FeedItem struct {
	Seq int64 `json:"seq"`
	Message
}

// remember adds m to the feed. It runs for every alert an enabled rule raises,
// whether or not email or the webhook is on, so an app in front of StunSails
// (the socket, see api/proxy.go) sees the same alerts the channels would.
func (n *Notifier) remember(m Message) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.seq++
	n.feed = append(n.feed, FeedItem{Seq: n.seq, Message: m})
	if len(n.feed) > feedSize {
		n.feed = append(n.feed[:0:0], n.feed[len(n.feed)-feedSize:]...)
	}
}

// Feed returns the alerts after seq, oldest first, and the newest seq.
func (n *Notifier) Feed(after int64) ([]FeedItem, int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := []FeedItem{}
	for _, it := range n.feed {
		if it.Seq > after {
			out = append(out, it)
		}
	}
	return out, n.seq
}
