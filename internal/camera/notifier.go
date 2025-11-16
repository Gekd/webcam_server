package camera

import "sync"

type notifier struct {
	mu  sync.Mutex
	ch  chan struct{}
	seq uint64
}

func newNotifier() *notifier {
	return &notifier{ch: make(chan struct{})}
}

func (n *notifier) next() {
	n.mu.Lock()
	n.seq++
	close(n.ch)
	n.ch = make(chan struct{})
	n.mu.Unlock()
}

func (n *notifier) Seq() uint64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.seq
}

func (n *notifier) WaitNext(since uint64) <-chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	if since != n.seq {
		// already advanced: return closed channel
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return n.ch
}
