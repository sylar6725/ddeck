package dd

import (
	"strings"
	"testing"

	"ddeck/internal/disk"
)

func TestOptimize(t *testing.T) {
	nvme := disk.Disk{Name: "nvme0n1", Path: "/dev/nvme0n1", Tran: "nvme", Type: "disk", PhySec: 512}
	ssd := disk.Disk{Name: "sda", Path: "/dev/sda", Tran: "sata", Type: "disk"}
	usb := disk.Disk{Name: "sdb", Path: "/dev/sdb", Tran: "usb", Type: "disk", Rota: true}

	cases := []struct {
		src, dst disk.Disk
		bs       string
		idirect  bool
	}{
		{nvme, nvme, "bs=16M", true},
		{nvme, ssd, "bs=8M", true},
		{nvme, usb, "bs=4M", false},
		{usb, ssd, "bs=4M", false},
	}
	for _, c := range cases {
		p := Optimize(c.src, c.dst)
		cmd := p.Command()
		if !strings.Contains(cmd, c.bs) || !strings.Contains(cmd, "status=progress") || !strings.Contains(cmd, "oflag=direct") {
			t.Errorf("%s", cmd)
		}
		if strings.Contains(cmd, "iflag=fullblock,direct") != c.idirect {
			t.Errorf("iflag direct attendu=%v : %s", c.idirect, cmd)
		}
	}
	if FormatBS(4096) != "4K" || FormatBS(1000) != "1000" {
		t.Error("FormatBS")
	}
}
