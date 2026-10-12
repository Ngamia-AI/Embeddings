package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

func goodApp() domain.App {
	return domain.App{
		Name:       "example-app",
		Status:     "active",
		Modalities: []string{"text", "image"},
		RPS:        10,
		Burst:      20,
		MediaRPS:   2,
		MaxBatch:   32,
		MaxMedia:   8,
	}
}

func TestNewKey(t *testing.T) {
	for _, class := range []string{"live", "bulk"} {
		plaintext, hash, prefix, err := newKey(class)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(plaintext, "emb_"+class+"_") {
			t.Fatalf("bad prefix: %s", plaintext)
		}
		want := sha256.Sum256([]byte(plaintext))
		if hex.EncodeToString(hash) != hex.EncodeToString(want[:]) {
			t.Fatalf("hash mismatch")
		}
		if !strings.HasPrefix(prefix, "emb_"+class+"_") || len(prefix) != len("emb_"+class+"_")+4 {
			t.Fatalf("bad display prefix: %q", prefix)
		}
	}
}

func TestNewKeyUnknownClassDefaultsLive(t *testing.T) {
	plaintext, _, _, err := newKey("whatever")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plaintext, "emb_live_") {
		t.Fatalf("class not defaulted: %s", plaintext)
	}
}

func TestValidate(t *testing.T) {
	missingName := goodApp()
	missingName.Name = ""
	if errs := Validate(&missingName, true); errs == nil {
		t.Fatal("want validation error for missing name")
	}

	good := goodApp()
	if errs := Validate(&good, true); errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}

	badModality := goodApp()
	badModality.Modalities = []string{"text", "nope"}
	if errs := Validate(&badModality, true); errs == nil {
		t.Fatal("want validation error for bad modality")
	}

	badRPS := goodApp()
	badRPS.RPS = 0
	if errs := Validate(&badRPS, true); errs == nil {
		t.Fatal("want validation error for rps")
	}
}
