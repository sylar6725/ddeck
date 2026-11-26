// Package dd choisit les options de dd et exécute les copies en suivant leur progression.
package dd

import (
	"fmt"
	"strings"

	"ddeck/internal/disk"
)

// Plan est la commande dd retenue pour une copie.
type Plan struct {
	Args []string
	BS   int64
	Desc string // explication courte, ex. « NVMe→USB bs=4M direct »
}

const mib = 1 << 20

// Optimize choisit taille de bloc et drapeaux selon le type des deux disques.
// Le disque le plus lent décide de la taille de bloc.
func Optimize(src, dst disk.Disk) Plan {
	sk, dk := src.Kind(), dst.Kind()
	slow := sk
	if dk < slow {
		slow = dk
	}

	var bs int64
	switch slow {
	case disk.KindNVMe:
		bs = 16 * mib
	case disk.KindSSD:
		bs = 8 * mib
	default: // HDD, USB, SD/MMC, loop
		bs = 4 * mib
	}

	sec := max64(max64(src.PhySec, dst.PhySec), max64(src.LogSec, dst.LogSec))
	if sec > 0 && bs%sec != 0 {
		bs = (bs/sec + 1) * sec
	}

	iflags := []string{"fullblock"}
	// Lecture directe seulement entre disques rapides : évite de polluer le cache.
	if fast(sk) && fast(dk) {
		iflags = append(iflags, "direct")
	}

	args := []string{
		"if=" + src.Path,
		"of=" + dst.Path,
		"bs=" + FormatBS(bs),
		"iflag=" + strings.Join(iflags, ","),
		"oflag=direct", // la progression reflète l'écriture réelle, pas le cache
		"conv=fsync",
		"status=progress",
	}
	desc := fmt.Sprintf("%s→%s bs=%s direct", sk, dk, FormatBS(bs))
	return Plan{Args: args, BS: bs, Desc: desc}
}

func fast(k disk.Kind) bool { return k == disk.KindNVMe || k == disk.KindSSD }

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// FormatBS écrit une taille de bloc au format dd (16M, 512K, 4096…).
func FormatBS(bs int64) string {
	switch {
	case bs%mib == 0:
		return fmt.Sprintf("%dM", bs/mib)
	case bs%1024 == 0:
		return fmt.Sprintf("%dK", bs/1024)
	}
	return fmt.Sprintf("%d", bs)
}

// Command renvoie la ligne de commande lisible.
func (p Plan) Command() string { return "dd " + strings.Join(p.Args, " ") }
