package dd

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

func TestParseProgress(t *testing.T) {
	b, s, ok := ParseProgress("1073741824 bytes (1.1 GB, 1.0 GiB) copied, 2.50123 s, 429 MB/s")
	if !ok || b != 1073741824 || s != 2.50123 {
		t.Fatalf("%d %f %v", b, s, ok)
	}
	if _, _, ok := ParseProgress("2048+0 records in"); ok {
		t.Error("ligne records acceptée")
	}
}

func TestScanCRLF(t *testing.T) {
	in := "10 bytes copied, 1 s, 10 B/s\r20 bytes copied, 2 s, 10 B/s\r\n2+0 records in\n"
	sc := bufio.NewScanner(strings.NewReader(in))
	sc.Split(scanCRLF)
	var got []int64
	for sc.Scan() {
		if b, _, ok := ParseProgress(sc.Text()); ok {
			got = append(got, b)
		}
	}
	if len(got) != 2 || got[1] != 20 {
		t.Fatalf("%v", got)
	}
}

func TestApplyETA(t *testing.T) {
	j := &Job{Total: 1000}
	j.Apply(ProgressMsg{Bytes: 100, Elapsed: 1})
	j.Apply(ProgressMsg{Bytes: 200, Elapsed: 2})
	if j.Speed != 100 {
		t.Fatalf("vitesse %f", j.Speed)
	}
	if j.ETA() != 8*time.Second || j.Percent() != 0.2 {
		t.Fatalf("eta %v pct %f", j.ETA(), j.Percent())
	}
}
