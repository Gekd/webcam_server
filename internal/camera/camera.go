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

	tk := newTicker(c.maxFPS)
	for c.run.Load() {
		tk.Wait()
		var jpegBytes []byte
		select {
		case jpegBytes = <-frames:
			if jpegBytes == nil {
				time.Sleep(30 * time.Millisecond)
				continue
			}
		default:
			time.Sleep(5 * time.Millisecond)
			continue
		}

		img, err := jpeg.Decode(bytes.NewReader(jpegBytes))
		if err != nil {
			log.Printf("[%s] jpeg decode error: %v", c.id, err)
			continue
		}
		rgba := toRGBA(img)
		w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()

		boxes, err := c.det.DetectJPEG(jpegBytes, 0.4, 0.45)
		if err != nil {
			log.Printf("[%s] detect error: %v", c.id, err)
		} else {
			rects := make([]image.Rectangle, 0, len(boxes))
			for _, b := range boxes {
				rects = append(rects, image.Rect(b.X1, b.Y1, b.X2, b.Y2))
			}
			drawBoxes(rgba, rects, color.RGBA{0, 255, 0, 255})
		}

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 85}); err != nil {
			log.Printf("[%s] jpeg encode error: %v", c.id, err)
			continue
		}

		c.mu.Lock()
		c.latest = buf.Bytes()
		c.szW, c.szH = w, h
		c.mu.Unlock()
		c.notif.next()
	}
}

func (c *Camera) LatestJPEG() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

func (c *Camera) Seq() uint64                           { return c.notif.Seq() }
func (c *Camera) WaitNext(since uint64) <-chan struct{} { return c.notif.WaitNext(since) }
