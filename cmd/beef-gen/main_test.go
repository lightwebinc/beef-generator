package main

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/lightwebinc/shard-common/objfmt"
)

// TestBuildObjectMarkers proves every synthetic encoding leads with a
// recognised BEEF-family marker and embeds the emission counter (unique
// ContentIDs across emissions).
func TestBuildObjectMarkers(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, enc := range []string{"beef", "beefv2", "atomic"} {
		a, err := buildObject(enc, 64, 0, rng)
		if err != nil {
			t.Fatalf("%s: %v", enc, err)
		}
		if !objfmt.IsBEEFObject(a) {
			t.Fatalf("%s: marker self-check failed", enc)
		}
		b, _ := buildObject(enc, 64, 1, rng)
		if objfmt.ContentID(a) == objfmt.ContentID(b) {
			t.Fatalf("%s: emissions 0 and 1 share a ContentID", enc)
		}
	}
	if _, err := buildObject("bogus", 64, 0, rng); err == nil {
		t.Fatal("unknown encoding accepted")
	}
}

// TestRealVectorIntegrity proves the embedded BRC-62 specification example
// decodes, carries the BEEF v1 marker, and survives the record round-trip
// verbatim — the verbatim-carriage fixture for scenario 94.
func TestRealVectorIntegrity(t *testing.T) {
	obj, err := hex.DecodeString(beefVectorHex)
	if err != nil {
		t.Fatalf("vector hex: %v", err)
	}
	if !objfmt.IsBEEFObject(obj) {
		t.Fatal("vector does not lead with a BEEF marker")
	}
	if w, _ := objfmt.BEEFVersionWord(obj); w != objfmt.BEEFMarkerV1 {
		t.Fatalf("vector version word %X, want BRC-62 v1", w)
	}

	rec, err := objfmt.EncodeBEEFRecord([]string{"tm_spec"}, obj)
	if err != nil {
		t.Fatalf("EncodeBEEFRecord: %v", err)
	}
	d, n, err := objfmt.DecodeBEEFRecord(rec)
	if err != nil || n != len(rec) {
		t.Fatalf("DecodeBEEFRecord: %v", err)
	}
	if string(d.Object) != string(obj) {
		t.Fatal("vector not carried verbatim through the record")
	}
}

// TestRecordSelfVerify mirrors the emitter's pre-write check.
func TestRecordSelfVerify(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	obj, _ := buildObject("beef", 128, 7, rng)
	rec, err := objfmt.EncodeBEEFRecord([]string{"tm_a", "tm_b"}, obj)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := objfmt.BEEFRecordSize(rec); err != nil || n != len(rec) {
		t.Fatalf("self-verify: n=%d err=%v", n, err)
	}
}
