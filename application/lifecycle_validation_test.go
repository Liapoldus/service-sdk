package application

import (
	"context"
	"errors"
	"testing"
)

func TestConfigurationValidationExposesOnlyItsScopedGeneration(t *testing.T) {
	lifecycle := &Lifecycle{}
	called := false
	err := lifecycle.ValidateConfigurationGeneration(context.Background(), "42", func() error {
		called = true
		generation, available := lifecycle.SecretGrantGeneration()
		if !available || generation != "42" {
			t.Fatalf("validation secret generation = %q, available=%v", generation, available)
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("validation callback: called=%v err=%v", called, err)
	}
	if generation, available := lifecycle.SecretGrantGeneration(); available || generation != "" {
		t.Fatalf("validation generation leaked beyond callback: %q, available=%v", generation, available)
	}
}

func TestConfigurationValidationRejectsNoncanonicalGeneration(t *testing.T) {
	for _, generation := range []string{"", "0", "01", "+1", "1.0", "abc"} {
		t.Run(generation, func(t *testing.T) {
			called := false
			err := (&Lifecycle{}).ValidateConfigurationGeneration(context.Background(), generation, func() error {
				called = true
				return nil
			})
			if !errors.Is(err, ErrInvalidValidationGeneration) || called {
				t.Fatalf("noncanonical generation accepted: called=%v err=%v", called, err)
			}
		})
	}
}
