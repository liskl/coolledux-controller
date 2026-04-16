// Package registry manages the set of BLE-connected CoolLEDUX panels the
// service is currently tracking. Each Entry owns its own BLE client,
// transport, and controller so REST and MQTT layers can route per-device
// by normalized MAC.
package registry

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
)

// Entry is one panel under management.
type Entry struct {
	// ID is the normalized MAC (lowercase, no colons). Stable identifier
	// used in REST paths and MQTT topics.
	ID string

	// Name is a human-friendly label from config, or the MAC if none was given.
	Name string

	// MAC is the canonical colon-separated BLE MAC ("01:00:00:FB:A4:16").
	MAC string

	Client     *ble.Client
	Transport  *ble.Transport
	Controller *controller.Controller
}

// Registry holds all Entry instances keyed by ID. Safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	order   []string // insertion order; first entry is the "primary" for legacy routes
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{
		entries: make(map[string]*Entry),
	}
}

// Add inserts an Entry. Returns an error if another entry with the same
// ID already exists — the caller should decide whether that's a
// duplicate-register (benign) or a config typo (warn).
func (r *Registry) Add(e *Entry) error {
	if e == nil {
		return fmt.Errorf("nil entry")
	}
	if e.ID == "" {
		return fmt.Errorf("entry missing ID")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[e.ID]; ok {
		return fmt.Errorf("device %s already registered", e.ID)
	}
	r.entries[e.ID] = e
	r.order = append(r.order, e.ID)
	return nil
}

// Get returns the entry for the given ID.
func (r *Registry) Get(id string) (*Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	return e, ok
}

// Primary returns the first registered entry, or nil if the registry is
// empty. Used to back the legacy single-device routes during migration.
func (r *Registry) Primary() *Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.order) == 0 {
		return nil
	}
	return r.entries[r.order[0]]
}

// List returns all entries in insertion order.
func (r *Registry) List() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Entry, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.entries[id])
	}
	return out
}

// IDs returns all registered IDs in sorted order. Handy for stable
// logging output when insertion order doesn't matter.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.entries))
	for id := range r.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Len returns the number of registered entries.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// BuildEntry wires a new Entry around a fresh BLE client, transport, and
// controller for the given device. It does NOT establish a BLE
// connection — the caller is responsible for calling Controller.Connect
// when ready. Split so tests can construct registries without real BLE.
func BuildEntry(dev config.DeviceConfig, cfg *config.Config, logger *slog.Logger) *Entry {
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	// The controller stores the MAC via cfg.BLE.DeviceMAC. For multi-device,
	// we derive each controller's own per-device config so Connect() hits
	// the right MAC without mutating the shared config.
	perDeviceCfg := *cfg
	perDeviceCfg.BLE = cfg.BLE
	perDeviceCfg.BLE.DeviceMAC = dev.MAC
	ctrl := controller.New(bleClient, transport, &perDeviceCfg, logger.With("device", dev.ID()))

	name := dev.Name
	if name == "" {
		name = dev.MAC
	}
	return &Entry{
		ID:         dev.ID(),
		Name:       name,
		MAC:        dev.MAC,
		Client:     bleClient,
		Transport:  transport,
		Controller: ctrl,
	}
}
