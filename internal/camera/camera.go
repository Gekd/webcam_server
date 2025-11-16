package camera

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"Garage48/internal/detector"
)

type Camera struct {
	id     string
	url    string
	maxFPS int
	detURL string
	det    *detector.Client

	mu     sync.RWMutex
	latest []byte
	szW    int
	szH    int

	// detection state
	lastBoxesMu sync.RWMutex
	lastBoxes   []detector.Box
	lastAt      time.Time

	detLatest atomic.Value // stores []byte (last frame to detect)

	notif *notifier
	run   atomic.Bool
}

func NewCamera(id, url, detectorURL string, maxFPS int) *Camera {
	return &Camera{
		id:     id,
		url:    url,
		maxFPS: maxFPS,
		detURL: detectorURL,
		det:    detector.New(detectorURL),
		notif:  newNotifier(),
	}
}

func (c *Camera) Start() {
	if !c.run.CompareAndSwap(false, true) {
		return
	}
	go c.loop()
}

func (c *Camera) Stop() {
	if !c.run.CompareAndSwap(true, false) {
		return
	}
}

func (c *Camera) loop() {
	u, _ := url.Parse(c.url)
	if u != nil && (u.Scheme == "http" || u.Scheme == "https") {
		c.loopMJPEG()
		return
	}
	log.Printf("[%s] unsupported url scheme (pure-Go expects http MJPEG): %s", c.id, c.url)
}

func (c *Camera) loopMJPEG() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewMJPEGClient(c.url)
	frames := make(chan []byte, 1)
	go func() {
		if err := client.Stream(ctx, frames); err != nil {
			log.Printf("[%s] MJPEG stream ended: %v", c.id, err)
		}
	}()

	// Start detector worker: always process the most recent frame, drop older
	go c.detectWorker(ctx, 150*time.Millisecond, 300*time.Millisecond)

	tk := newTicker(c.maxFPS)
	for c.run.Load() {
		tk.Wait()
		var jpegBytes []byte
		select {
		case jpegBytes = <-frames:
			if jpegBytes == nil {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			// publish current frame for detection (latest-only)
			c.detLatest.Store(jpegBytes)
		default:
			time.Sleep(5 * time.Millisecond)
			continue
		}

		// Decide: if we have fresh detections, draw; else pass-through
		boxes, fresh := c.getFreshBoxes(500 * time.Millisecond)
		if !fresh || len(boxes) == 0 {
			// Pass through original JPEG: minimal latency
			c.publish(jpegBytes, 0, 0)
			continue
		}

		// Draw boxes onto current frame
		img, err := jpeg.Decode(bytes.NewReader(jpegBytes))
		if err != nil {
			log.Printf("[%s] jpeg decode error: %v", c.id, err)
			// fallback to pass-through
			c.publish(jpegBytes, 0, 0)
			continue
		}
		rgba := toRGBA(img)
		w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()

		rects := make([]image.Rectangle, 0, len(boxes))
		for _, b := range boxes {
			rects = append(rects, image.Rect(b.X1, b.Y1, b.X2, b.Y2))
		}
		drawBoxes(rgba, rects, color.RGBA{0, 255, 0, 255})

		var buf bytes.Buffer
		// Lower quality a bit for speed
		if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 80}); err != nil {
			log.Printf("[%s] jpeg encode error: %v", c.id, err)
			// fallback to pass-through
			c.publish(jpegBytes, 0, 0)
			continue
		}
		c.publish(buf.Bytes(), w, h)
	}
}

func (c *Camera) detectWorker(ctx context.Context, interval time.Duration, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for c.run.Load() {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v := c.detLatest.Load()
			if v == nil {
				continue
			}
			jpg := v.([]byte)
			dctx, cancel := context.WithTimeout(ctx, timeout)
			boxes, err := c.det.DetectJPEGCtx(dctx, jpg, 0.4, 0.45)
			cancel()
			if err != nil {
				// keep last boxes; don't log every time to avoid spam
				continue
			}
			c.lastBoxesMu.Lock()
			c.lastBoxes = boxes
			c.lastAt = time.Now()
			c.lastBoxesMu.Unlock()
		}
	}
}

func (c *Camera) getFreshBoxes(maxAge time.Duration) ([]detector.Box, bool) {
	c.lastBoxesMu.RLock()
	defer c.lastBoxesMu.RUnlock()
	if len(c.lastBoxes) == 0 {
		return nil, false
	}
	if time.Since(c.lastAt) > maxAge {
		return nil, false
	}
	// copy slice header (boxes are small structs)
	out := make([]detector.Box, len(c.lastBoxes))
	copy(out, c.lastBoxes)
	return out, true
}

func (c *Camera) publish(jpg []byte, w, h int) {
	c.mu.Lock()
	c.latest = jpg
	if w > 0 && h > 0 {
		c.szW, c.szH = w, h
	}
	c.mu.Unlock()
	c.notif.next()
}

func (c *Camera) LatestJPEG() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

func (c *Camera) Seq() uint64                           { return c.notif.Seq() }
func (c *Camera) WaitNext(since uint64) <-chan struct{} { return c.notif.WaitNext(since) }
