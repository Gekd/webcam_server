package camera

import "sync"

// CameraFactory creates a Camera from id and url.
type CameraFactory func(id, url string) *Camera

type Registry struct {
	mu      sync.RWMutex
	cameras map[string]*Camera
	factory CameraFactory
}

func NewRegistry(factory CameraFactory) *Registry {
	return &Registry{
		cameras: make(map[string]*Camera),
		factory: factory,
	}
}

func (r *Registry) AddCamera(id, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.cameras[id]; exists {
		return nil
	}
	c := r.factory(id, url)
	if c == nil {
		return nil
	}
	r.cameras[id] = c
	c.Start()
	return nil
}

func (r *Registry) Get(id string) *Camera {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cameras[id]
}

func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.cameras {
		c.Stop()
	}
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.cameras))
	for id := range r.cameras {
		out = append(out, id)
	}
	return out
}
