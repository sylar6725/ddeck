package disk

import "testing"

const fixture = `{"blockdevices":[
 {"name":"nvme0n1","path":"/dev/nvme0n1","size":500107862016,"type":"disk","tran":"nvme","rota":false,"rm":false,"model":"Samsung SSD 980","serial":"S1","fstype":null,"pttype":"gpt","phy-sec":512,"log-sec":512,"mountpoints":[null],
  "children":[
   {"name":"nvme0n1p1","path":"/dev/nvme0n1p1","size":536870912,"type":"part","fstype":"vfat","mountpoints":["/boot/efi"]},
   {"name":"nvme0n1p2","path":"/dev/nvme0n1p2","size":499570991104,"type":"part","fstype":"crypto_LUKS","mountpoints":[null],
    "children":[{"name":"root","path":"/dev/mapper/root","size":1,"type":"crypt","fstype":"ext4","mountpoints":["/"]}]}
  ]},
 {"name":"sdb","path":"/dev/sdb","size":"64000000000","type":"disk","tran":"usb","rota":"1","rm":"1","model":"DataTraveler","serial":null,"fstype":null,"pttype":null,"phy-sec":"512","log-sec":"512","mountpoint":null},
 {"name":"sda","path":"/dev/sda","size":2000398934016,"type":"disk","tran":"sata","rota":true,"rm":false,"model":"WDC","pttype":"dos","phy-sec":4096,"log-sec":512,"mountpoints":[null]},
 {"name":"loop0","path":"/dev/loop0","size":123,"type":"loop","fstype":"squashfs","mountpoints":["/snap/core/1"]},
 {"name":"zram0","path":"/dev/zram0","size":8000,"type":"disk","mountpoints":["[SWAP]"]},
 {"name":"sr0","path":"/dev/sr0","size":1024,"type":"rom"}
]}`

func TestParse(t *testing.T) {
	disks, err := Parse([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(disks) != 3 {
		t.Fatalf("attendu 3 disques, obtenu %d", len(disks))
	}
	nv, usb, hdd := disks[0], disks[1], disks[2]
	if nv.Kind() != KindNVMe || usb.Kind() != KindUSB || hdd.Kind() != KindHDD {
		t.Errorf("types : %v %v %v", nv.Kind(), usb.Kind(), hdd.Kind())
	}
	if !nv.IsSystem() || !nv.Mounted() {
		t.Error("le NVMe porte / (via LUKS) : doit être système")
	}
	if nv.Parts[1].FSType != "crypto_LUKS" {
		t.Errorf("fs profond : %q", nv.Parts[1].FSType)
	}
	if usb.Mounted() || usb.IsSystem() || !usb.Removable || usb.Size != 64000000000 {
		t.Errorf("usb mal lu : %+v", usb)
	}
	if hdd.PhySec != 4096 {
		t.Errorf("phy-sec : %d", hdd.PhySec)
	}
}

func TestContentSummary(t *testing.T) {
	if s := (Content{Checked: true}).Summary(); s != "empty" {
		t.Error(s)
	}
	c := Content{Checked: true, PTType: "dos", FSTypes: []string{"ntfs"}}
	if !c.HasData() || c.Empty() || c.Summary() != "mbr: ntfs" {
		t.Error(c.Summary())
	}
	if (Content{}).Empty() {
		t.Error("non vérifié ne doit pas être considéré vide")
	}
}
