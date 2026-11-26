// Package disk détecte les disques de stockage via lsblk et inspecte leur contenu.
package disk

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"ddeck/internal/i18n"
)

// Kind est la famille matérielle d'un disque.
type Kind int

const (
	KindOther Kind = iota
	KindUSB
	KindMMC
	KindHDD
	KindLoop
	KindSSD
	KindNVMe
)

func (k Kind) String() string {
	switch k {
	case KindUSB:
		return "USB"
	case KindMMC:
		return "SD/MMC"
	case KindHDD:
		return "HDD"
	case KindLoop:
		return "LOOP"
	case KindSSD:
		return "SSD"
	case KindNVMe:
		return "NVMe"
	}
	return "?"
}

// Part est une partition (ou un périphérique enfant) d'un disque.
type Part struct {
	Path        string
	FSType      string
	Size        int64
	Mountpoints []string // inclut les points de montage des descendants (LVM, LUKS…)
}

// Disk est un disque entier détecté.
type Disk struct {
	Name        string
	Path        string
	Type        string // disk | loop
	Model       string
	Serial      string
	Tran        string // sata, nvme, usb, mmc…
	Size        int64
	Rota        bool
	Removable   bool
	PhySec      int64
	LogSec      int64
	PTType      string
	FSType      string
	Mountpoints []string
	Parts       []Part
}

// Kind déduit la famille du disque.
func (d Disk) Kind() Kind {
	switch {
	case d.Type == "loop":
		return KindLoop
	case d.Tran == "nvme" || strings.HasPrefix(d.Name, "nvme"):
		return KindNVMe
	case d.Tran == "usb":
		return KindUSB
	case d.Tran == "mmc" || strings.HasPrefix(d.Name, "mmcblk"):
		return KindMMC
	case d.Rota:
		return KindHDD
	}
	return KindSSD
}

// AllMountpoints renvoie tous les points de montage du disque et de ses partitions.
func (d Disk) AllMountpoints() []string {
	out := append([]string{}, d.Mountpoints...)
	for _, p := range d.Parts {
		out = append(out, p.Mountpoints...)
	}
	return out
}

// Mounted indique si le disque ou une de ses partitions est monté (ou utilisé en swap).
func (d Disk) Mounted() bool { return len(d.AllMountpoints()) > 0 }

// IsSystem indique si le disque porte le système en cours d'exécution.
func (d Disk) IsSystem() bool {
	for _, m := range d.AllMountpoints() {
		switch m {
		case "/", "/boot", "/boot/efi", "/usr", "/var", "[SWAP]":
			return true
		}
	}
	return false
}

// Label est un nom court lisible : modèle ou nom du périphérique.
func (d Disk) Label() string {
	if d.Model != "" {
		return d.Model
	}
	return d.Name
}

// ---- parsing lsblk ----

// flexInt accepte un nombre JSON, une chaîne numérique ou null (selon la version de lsblk).
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

// flexBool accepte true/false, "1"/"0" ou null.
type flexBool bool

func (f *flexBool) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*f = flexBool(s == "true" || s == "1")
	return nil
}

type lsblkDev struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Type        string     `json:"type"`
	Model       *string    `json:"model"`
	Serial      *string    `json:"serial"`
	Tran        *string    `json:"tran"`
	Size        flexInt    `json:"size"`
	Rota        flexBool   `json:"rota"`
	RM          flexBool   `json:"rm"`
	PhySec      flexInt    `json:"phy-sec"`
	LogSec      flexInt    `json:"log-sec"`
	PTType      *string    `json:"pttype"`
	FSType      *string    `json:"fstype"`
	Mountpoint  *string    `json:"mountpoint"`
	Mountpoints []*string  `json:"mountpoints"`
	Children    []lsblkDev `json:"children"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func (l lsblkDev) mounts() []string {
	var out []string
	if m := str(l.Mountpoint); m != "" {
		out = append(out, m)
	}
	for _, m := range l.Mountpoints {
		if s := str(m); s != "" && (len(out) == 0 || out[0] != s) {
			out = append(out, s)
		}
	}
	return out
}

// mountsDeep renvoie les montages du nœud et de tous ses descendants.
func (l lsblkDev) mountsDeep() []string {
	out := l.mounts()
	for _, c := range l.Children {
		out = append(out, c.mountsDeep()...)
	}
	return out
}

// fsDeep renvoie les systèmes de fichiers des descendants (pour les partitions chiffrées/LVM).
func (l lsblkDev) fsDeep() string {
	if fs := str(l.FSType); fs != "" {
		return fs
	}
	for _, c := range l.Children {
		if fs := c.fsDeep(); fs != "" {
			return fs
		}
	}
	return ""
}

const lsblkCols = "NAME,PATH,SIZE,TYPE,TRAN,ROTA,RM,MODEL,SERIAL,FSTYPE,PTTYPE,PHY-SEC,LOG-SEC"

// List interroge lsblk et renvoie les disques entiers exploitables.
func List() ([]Disk, error) {
	out, err := exec.Command("lsblk", "-J", "-b", "-o", lsblkCols+",MOUNTPOINTS").Output()
	if err != nil {
		// util-linux < 2.37 ne connaît pas MOUNTPOINTS
		out, err = exec.Command("lsblk", "-J", "-b", "-o", lsblkCols+",MOUNTPOINT").Output()
		if err != nil {
			return nil, fmt.Errorf("lsblk: %w", err)
		}
	}
	return Parse(out)
}

// Parse convertit la sortie JSON de lsblk en disques.
func Parse(data []byte) ([]Disk, error) {
	var root struct {
		Blockdevices []lsblkDev `json:"blockdevices"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf(i18n.S("reading lsblk JSON: %w", "lecture JSON lsblk : %w"), err)
	}
	var disks []Disk
	for _, l := range root.Blockdevices {
		if l.Type != "disk" && l.Type != "loop" {
			continue
		}
		if l.Size <= 0 || strings.HasPrefix(l.Name, "zram") || strings.HasPrefix(l.Name, "ram") {
			continue
		}
		if l.Type == "loop" && str(l.FSType) == "squashfs" { // snaps
			continue
		}
		path := l.Path
		if path == "" {
			path = "/dev/" + l.Name
		}
		d := Disk{
			Name:        l.Name,
			Path:        path,
			Type:        l.Type,
			Model:       str(l.Model),
			Serial:      str(l.Serial),
			Tran:        strings.ToLower(str(l.Tran)),
			Size:        int64(l.Size),
			Rota:        bool(l.Rota),
			Removable:   bool(l.RM),
			PhySec:      int64(l.PhySec),
			LogSec:      int64(l.LogSec),
			PTType:      str(l.PTType),
			FSType:      str(l.FSType),
			Mountpoints: l.mounts(),
		}
		for _, c := range l.Children {
			p := c.Path
			if p == "" {
				p = "/dev/" + c.Name
			}
			d.Parts = append(d.Parts, Part{
				Path:        p,
				FSType:      c.fsDeep(),
				Size:        int64(c.Size),
				Mountpoints: c.mountsDeep(),
			})
		}
		disks = append(disks, d)
	}
	return disks, nil
}
