package drivers

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	log "github.com/sirupsen/logrus"
)

const (
	EfsTemplateURI = "%s.efs.%s.amazonaws.com"
)

type efsDriver struct {
	volumeDriver
	resolve  bool
	region   string
	resolver *Resolver
	dnscache map[string]string
}

func NewEFSDriver(root, nameserver string, resolve bool, mounts *MountManager) (efsDriver, error) {

	d := efsDriver{
		volumeDriver: newVolumeDriver(root, mounts),
		resolve:      resolve,
		dnscache:     map[string]string{},
	}

	if resolve {
		d.resolver = NewResolver(nameserver)
	}
	md, err := fetchAWSMetaData()
	if err != nil {
		return d, fmt.Errorf("error resolving AWS metadata: %w", err)
	}
	d.region = md.Region
	return d, nil
}

func (e efsDriver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	e.m.Lock()
	defer e.m.Unlock()
	hostdir := mountpoint(e.root, r.Name)
	source := e.fixSource(r.Name, r.ID)

	if e.mountm.HasMount(r.Name) && e.mountm.Count(r.Name) > 0 {
		log.Infof("Using existing EFS volume mount: %s", hostdir)
		e.mountm.Increment(r.Name)
		if isMounted(hostdir) {
			return &volume.MountResponse{Mountpoint: hostdir}, nil
		}
		log.Infof("Existing EFS volume not mounted, force remount.")
		e.mountm.Decrement(r.Name)
	}

	log.Infof("Mounting EFS volume %s on %s", source, hostdir)

	if err := createDest(hostdir); err != nil {
		return nil, err
	}

	if err := e.mountVolume(source, hostdir); err != nil {
		os.Remove(hostdir)
		return nil, err
	}
	e.mountm.Add(r.Name, hostdir)
	return &volume.MountResponse{Mountpoint: hostdir}, nil
}

func (e efsDriver) Unmount(r *volume.UnmountRequest) error {
	e.m.Lock()
	defer e.m.Unlock()
	hostdir := mountpoint(e.root, r.Name)

	if e.mountm.HasMount(r.Name) {
		if e.mountm.Count(r.Name) > 1 {
			log.Infof("Skipping unmount for %s - in use by other containers", hostdir)
			e.mountm.Decrement(r.Name)
			return nil
		}
		e.mountm.Decrement(r.Name)
	}

	log.Infof("Unmounting volume %s from %s", r.Name, hostdir)

	if err := runUmount(hostdir); err != nil {
		return err
	}

	e.mountm.DeleteIfNotManaged(r.Name)

	// Remove the mountpoint directory (not the volume name!)
	if err := os.Remove(hostdir); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to remove mountpoint directory %s: %v", hostdir, err)
	}

	return nil
}

func (e efsDriver) fixSource(name, id string) string {
	if e.mountm.HasOption(name, ShareOpt) {
		name = e.mountm.GetOption(name, ShareOpt)
	}

	v := strings.Split(name, "/")
	reg, _ := regexp.Compile("([0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+)$")
	uri := reg.FindString(v[0])

	if e.resolve {
		uri = fmt.Sprintf(EfsTemplateURI, v[0], e.region)
		if i, ok := e.dnscache[uri]; ok {
			uri = i
		}

		log.Debugf("Attempting to resolve: %s", uri)
		if ip, err := e.resolver.Lookup(uri); err == nil {
			log.Debugf("Resolved Addresses: %s", ip)
			e.dnscache[uri] = ip
			uri = ip
		} else {
			log.Errorf("Error during resolve: %s", err.Error())
		}
	}
	v[0] = uri + ":"
	return strings.Join(v, "/")
}

func (e efsDriver) mountVolume(source, dest string) error {
	log.Debugf("exec: mount -t nfs4 -o nfsvers=4.1 %s %s", source, dest)
	return runMount("nfs4", "nfsvers=4.1", source, dest)
}
