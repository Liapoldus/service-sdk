package presentation

import (
	"context"
	"testing"

	"github.com/Liapoldus/plugin-sdk/v2/domain/models"
)

func TestManifestProviderReturnsDefensiveCopies(t *testing.T) {
	provider, err := NewManifestProvider(models.ManifestDescriptor{ID: "echo", Version: "1.0.0"}, []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manifest[0] = 'x'
	again, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again[0] == 'x' {
		t.Fatal("provider exposed mutable manifest bytes")
	}
}
