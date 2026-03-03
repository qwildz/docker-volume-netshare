package drivers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-plugins-helpers/volume"
	log "github.com/sirupsen/logrus"
)

const (
	ShareOpt  = "share"
	CreateOpt = "create"
)

type mount struct {
	name        string
	hostdir     string
	connections int
	opts        map[string]string
	managed     bool
}

// MountManager tracks volume mount state with internal synchronization.
// All public methods are safe for concurrent use.
type MountManager struct {
	mu     sync.RWMutex
	mounts map[string]*mount
}

func NewVolumeManager() *MountManager {
	return &MountManager{
		mounts: map[string]*mount{},
	}
}

func (m *MountManager) HasMount(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, found := m.mounts[name]
	return found
}

func (m *MountManager) HasOptions(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hasOptionsLocked(name)
}

func (m *MountManager) hasOptionsLocked(name string) bool {
	c, found := m.mounts[name]
	if found {
		return c.opts != nil && len(c.opts) > 0
	}
	return false
}

func (m *MountManager) HasOption(name, key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hasOptionLocked(name, key)
}

func (m *MountManager) hasOptionLocked(name, key string) bool {
	c, found := m.mounts[name]
	if found && c.opts != nil {
		_, ok := c.opts[key]
		return ok
	}
	return false
}

func (m *MountManager) GetOptions(name string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.hasOptionsLocked(name) {
		c := m.mounts[name]
		return c.opts
	}
	return map[string]string{}
}

func (m *MountManager) GetOption(name, key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getOptionLocked(name, key)
}

func (m *MountManager) getOptionLocked(name, key string) string {
	if m.hasOptionLocked(name, key) {
		return m.mounts[name].opts[key]
	}
	return ""
}

func (m *MountManager) GetOptionAsBool(name, key string) bool {
	rv := strings.ToLower(m.GetOption(name, key))
	return rv == "yes" || rv == "true"
}

func (m *MountManager) IsActiveMount(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, found := m.mounts[name]
	return found && c.connections > 0
}

func (m *MountManager) Count(name string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.countLocked(name)
}

func (m *MountManager) countLocked(name string) int {
	c, found := m.mounts[name]
	if found {
		return c.connections
	}
	return 0
}

func (m *MountManager) Add(name, hostdir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, found := m.mounts[name]; found {
		m.incrementLocked(name)
	} else {
		m.mounts[name] = &mount{name: name, hostdir: hostdir, managed: false, connections: 1}
	}
}

func (m *MountManager) Create(name, hostdir string, opts map[string]string) *mount {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, found := m.mounts[name]
	if found && c.connections > 0 {
		c.opts = opts
		return c
	}
	mnt := &mount{name: name, hostdir: hostdir, managed: true, opts: opts, connections: 0}
	m.mounts[name] = mnt
	return mnt
}

// Delete removes a volume if it has no active connections and no container references.
// Docker API calls are made outside the lock to avoid blocking other operations.
func (m *MountManager) Delete(name string) error {
	// Quick check under read lock
	m.mu.RLock()
	mnt, found := m.mounts[name]
	if !found {
		m.mu.RUnlock()
		return nil
	}
	if mnt.connections >= 1 {
		m.mu.RUnlock()
		return errors.New("Volume is currently in use")
	}
	m.mu.RUnlock()

	// Check Docker API for container references OUTSIDE the lock
	refCount, err := checkReferences(name)
	if err != nil {
		log.Errorf("Error checking volume references for %s: %v. Assuming volume is in use for safety.", name, err)
		return fmt.Errorf("failed to check volume references: %w", err)
	}
	log.Debugf("Reference count for %s: %d", name, refCount)

	if refCount >= 1 {
		return errors.New("Volume is currently in use")
	}

	// Acquire write lock and delete (re-check under lock in case state changed)
	m.mu.Lock()
	defer m.mu.Unlock()
	mnt, found = m.mounts[name]
	if !found {
		return nil
	}
	if mnt.connections >= 1 {
		return errors.New("Volume is currently in use")
	}

	log.Debugf("Delete volume: %s, connections: %d", name, mnt.connections)
	delete(m.mounts, name)
	return nil
}

func (m *MountManager) DeleteIfNotManaged(name string) error {
	m.mu.RLock()
	mnt, found := m.mounts[name]
	if !found || mnt.connections > 0 || mnt.managed {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	log.Infof("Removing un-managed volume: %s", name)
	return m.Delete(name)
}

func (m *MountManager) Increment(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.incrementLocked(name)
}

func (m *MountManager) incrementLocked(name string) int {
	c, found := m.mounts[name]
	if found {
		log.Infof("Incrementing for %s: %d -> %d", name, c.connections, c.connections+1)
		c.connections++
		return c.connections
	}
	return 0
}

func (m *MountManager) Decrement(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.decrementLocked(name)
}

func (m *MountManager) decrementLocked(name string) int {
	c, found := m.mounts[name]
	if found && c.connections > 0 {
		log.Infof("Decrementing for %s: %d -> %d", name, c.connections, c.connections-1)
		c.connections--
		return c.connections
	}
	return 0
}

func (m *MountManager) GetVolumes(rootPath string) []*volume.Volume {
	m.mu.RLock()
	defer m.mu.RUnlock()

	volumes := []*volume.Volume{}
	for _, mount := range m.mounts {
		volumes = append(volumes, &volume.Volume{Name: mount.name, Mountpoint: mount.hostdir})
	}
	return volumes
}

func (m *MountManager) AddMount(name string, hostdir string, connections int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mounts[name] = &mount{name: name, hostdir: hostdir, managed: true, connections: connections}
}

// checkReferences queries Docker for containers (running + stopped) referencing the volume.
// Returns the count and any error instead of calling log.Fatal.
func checkReferences(volumeName string) (int, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 0, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containerList, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w (use -a flag to setup the DOCKER_API_VERSION)", err)
	}

	var counter int
	for _, ctr := range containerList {
		for _, mnt := range ctr.Mounts {
			if mnt.Name == volumeName {
				counter++
			}
		}
	}
	return counter, nil
}
