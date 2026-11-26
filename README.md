# ddeck — disk cloning with `dd`, in a TUI

🇫🇷 [Version française](README.fr.md)

ddeck is a terminal user interface (Linux) for `dd`:

```
╭─ ① SOURCE ──────────────────╮╭─ ② DESTINATION ─────────────╮
│ ▸ /dev/nvme0n1  NVMe 500 GB ││ ▸ /dev/sdb  USB  512 GB     │
│   ● data (gpt: ext4)        ││   ○ empty  Kingston         │
╰─────────────────────────────╯╰─────────────────────────────╯
╭─ COPIES ───────────────────────────────────────────────────╮
│ #1  /dev/nvme0n1 → /dev/sdb                    ● Running   │
│     [███████████████░░░░░░░░░░]  63%                       │
│     312.4 GB / 500.1 GB   412 MB/s   ETA 02:15   elapsed … │
╰────────────────────────────────────────────────────────────╯
```

The interface is in English by default. Press `l` (or `F2`) at any time to switch to French and back.

## Why use ddeck instead of plain `dd`?

`dd` is powerful but unforgiving: one typo in `of=` and the wrong disk is wiped, with no warning. ddeck keeps the power of `dd` and adds the safety and comfort it lacks.

- **No device path to type**: you pick disks from a list showing model, size, type (NVMe, SSD, HDD, USB, SD) and serial number. No more mixing up `/dev/sdb` and `/dev/sdc`.
- **Protection against the classic mistake**: swapping source and destination. ddeck looks at what is really on both disks. A source that looks empty, or a destination that holds data, means you must type a confirmation word.
- **Dangerous copies are simply impossible**: the system disk, a mounted disk or a disk that is too small can never be chosen as destination.
- **Good settings, no expertise needed**: block size and flags are chosen for the hardware. Copies are faster than with `dd`'s default 512-byte block, and the data is really written to disk (`conv=fsync`) before ddeck says "done".
- **Real progress**: percentage, speed, time remaining (ETA) and elapsed time, instead of `dd`'s raw byte counter. Because of `oflag=direct`, the progress shows what is really written, not what sits in the cache.
- **Several clones at once**: start a copy, then another one right away, and follow them all in the same window. Disks already in use cannot be selected twice.
- **Clean cancel**: cancel one copy (`x`) without stopping the others. Quitting while copies are running asks first.
- **Transparent**: the exact `dd` command is shown before every copy. You know what will run and can reuse it in a script.
- **Easy to deploy**: one static binary with no dependencies. The target machine only needs `lsblk` and `dd`, which nearly every Linux system already has. Without root, ddeck still runs in read-only mode to look at the disks.
- **English or French**: switch at any time with `l`.

## What it does

- **Three panels**: source at the top left, destination at the top right (the source disk is left out), copies at the bottom. Each copy takes 3 lines: the two disks, the progress bar, then bytes copied, speed, ETA, elapsed time and the options used.
- **Detects** every disk through `lsblk`: SATA, NVMe, USB, SD/MMC and loop devices. Snaps and zram are skipped.
- **Guards against swapping source and destination**: before starting, ddeck reads both disks again. It looks for a partition table and filesystems, and reads raw samples at the start, middle and end of the disk. If the source looks empty, or the destination is not empty, you must type `YES` (or `OUI` in French) to go ahead anyway.
- **Hard blocks that cannot be overridden**: not running as root, destination mounted, destination too small, or destination is the system disk.
- **`dd` options matched to the hardware**: the slower of the two disks sets the block size (see the table below).
- **Several copies at once**: once a copy starts, you are back at source selection. Disks already used by a running copy cannot be selected.

## Workflow

1. **Source** (top-left panel): pick the disk to copy.
2. **Destination** (top-right panel): pick the disk to overwrite. The system disk, disks that are too small and disks already in use by a copy cannot be selected.
3. **Confirmation**: a summary of both disks, the checks and the chosen `dd` command. Press `Y` to start, or type `YES` + Enter when a possible swap was detected.
4. **Copies** (bottom panel): follow the copy. You can start another one right away; it is added below.

## dd options by hardware

| Slowest disk | bs  | Direct read |
|--------------|-----|-------------|
| NVMe         | 16M | yes (if both disks are SSD/NVMe) |
| SATA SSD     | 8M  | yes (if both disks are SSD/NVMe) |
| HDD/USB/SD   | 4M  | no          |

Always set: `iflag=fullblock oflag=direct conv=fsync status=progress`.

## Installation

Requires Go ≥ 1.22. From Windows, macOS or Linux:

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=-s -o ddeck .
```

This produces a static binary with no dependencies. The target machine only needs `lsblk` and `dd`.

```
sudo ./ddeck
```

## Keys

| Key     | Action |
|---------|--------|
| ↑/↓ j/k | move |
| Enter   | select |
| Esc     | previous step |
| Tab     | move focus to the Copies panel and back |
| x       | cancel the selected copy (Copies panel) |
| d       | remove a finished copy from the list |
| r       | rescan disks |
| l / F2  | switch language English ⇄ French (`F2` also works while typing `YES`) |
| q       | quit (asks first if copies are running) |

## Safe testing with loop devices

```
truncate -s 512M src.img dst.img
sudo losetup -fP --show src.img     # → /dev/loopX
sudo losetup -fP --show dst.img     # → /dev/loopY
sudo parted -s /dev/loopX mklabel gpt mkpart p ext4 1MiB 100%
sudo mkfs.ext4 /dev/loopXp1
sudo ./ddeck                        # loopX → loopY
cmp src.img dst.img && echo identical
```

## Development

```
go vet ./... && go test ./...
```

- [internal/disk/](internal/disk/): detection (`lsblk`) and content inspection
- [internal/dd/](internal/dd/): option selection and running `dd`
- [internal/ui/](internal/ui/): Bubble Tea interface
- [internal/i18n/](internal/i18n/): current language (English/French)
