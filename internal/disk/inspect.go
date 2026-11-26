package disk

import (
	"os"
	"strings"

	"ddeck/internal/i18n"
)

// Content décrit ce que contient un disque.
type Content struct {
	Checked bool     // la lecture brute a réussi
	PTType  string   // table de partitions (gpt, dos…)
	FSTypes []string // systèmes de fichiers détectés
	NonZero bool     // des octets non nuls ont été trouvés dans les échantillons
	ReadErr string   // erreur de lecture brute (souvent : pas root)
}

// HasData est vrai si le disque contient manifestement quelque chose.
func (c Content) HasData() bool {
	return c.PTType != "" || len(c.FSTypes) > 0 || c.NonZero
}

// Empty est vrai seulement si l'on a pu vérifier que le disque est vide.
func (c Content) Empty() bool { return c.Checked && !c.HasData() }

// Summary décrit le contenu en quelques mots.
func (c Content) Summary() string {
	if !c.HasData() {
		if !c.Checked {
			return i18n.S("unknown content", "contenu inconnu")
		}
		return i18n.S("empty", "vide")
	}
	pt := c.PTType
	if pt == "dos" {
		pt = "mbr"
	}
	fs := strings.Join(c.FSTypes, ", ")
	switch {
	case pt != "" && fs != "":
		return pt + ": " + fs
	case pt != "":
		return pt
	case fs != "":
		return fs
	}
	return i18n.S("raw data", "données brutes")
}

const sampleSize = 1 << 20

// Inspect analyse un disque : métadonnées lsblk puis lecture d'échantillons bruts
// (début, milieu, fin) pour détecter des données non nulles.
func Inspect(d Disk) Content {
	c := Content{PTType: d.PTType}
	seen := map[string]bool{}
	add := func(fs string) {
		if fs != "" && !seen[fs] {
			seen[fs] = true
			c.FSTypes = append(c.FSTypes, fs)
		}
	}
	add(d.FSType)
	for _, p := range d.Parts {
		add(p.FSType)
	}

	f, err := os.Open(d.Path)
	if err != nil {
		c.ReadErr = err.Error()
		return c
	}
	defer f.Close()

	buf := make([]byte, sampleSize)
	offsets := []int64{0}
	if d.Size > 3*sampleSize {
		offsets = append(offsets, (d.Size/2)&^4095, d.Size-sampleSize)
	}
	for _, off := range offsets {
		n, err := f.ReadAt(buf, off)
		if n == 0 && err != nil {
			c.ReadErr = err.Error()
			return c
		}
		for _, b := range buf[:n] {
			if b != 0 {
				c.NonZero = true
				break
			}
		}
		if c.NonZero {
			break
		}
	}
	c.Checked = true
	return c
}
