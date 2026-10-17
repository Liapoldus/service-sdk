package infrastructure

import (
	"context"
	"errors"
	"time"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

const (
	defaultReplicaRetryInitial = 250 * time.Millisecond
	defaultReplicaRetryMaximum = 5 * time.Second
)

// ReplicaLifecycleOperations is the narrow client surface needed to keep one
// immutable replica registration leased. ReplicaLifecycleClient implements it.
type ReplicaLifecycleOperations interface {
	Register(context.Context, models.ReplicaRegistrationRequest) (models.ReplicaRegistrationResponse, error)
	Renew(context.Context, models.ReplicaRenewalRequest) (models.ReplicaLease, error)
}

// ReplicaLeaseMaintenance describes one service-owned registration and its
// current SDK readiness. The manager never changes immutable endpoint, release,
// contract or identity metadata; it only refreshes applied generation/readiness.
type ReplicaLeaseMaintenance struct {
	Client        ReplicaLifecycleOperations
	Registration  models.ReplicaRegistrationRequest
	Readiness     func() models.Readiness
	RenewInterval time.Duration
	RetryInitial  time.Duration
	RetryMaximum  time.Duration
}

// MaintainReplicaLease performs the initial registration, renews the lease,
// retries transient Core unavailability, and re-registers the same immutable
// incarnation if a Core restart has lost its in-memory replica directory.
// Lease expiry and other terminal refusals are returned without changing the
// identity or attempting to create a new incarnation. Cancellation is a
// graceful stop and returns nil.
func MaintainReplicaLease(ctx context.Context, maintenance ReplicaLeaseMaintenance) error {
	if ctx == nil || maintenance.Client == nil || maintenance.Readiness == nil ||
		maintenance.RenewInterval <= 0 || maintenance.Registration.Validate() != nil {
		return models.ErrInvalidReplicaRegistration
	}
	initial, maximum := maintenance.RetryInitial, maintenance.RetryMaximum
	if initial <= 0 {
		initial = defaultReplicaRetryInitial
	}
	if maximum <= 0 {
		maximum = defaultReplicaRetryMaximum
	}
	if maximum < initial {
		return models.ErrInvalidReplicaRegistration
	}

	if err := registerUntilAvailable(ctx, maintenance, initial, maximum); err != nil || ctx.Err() != nil {
		return err
	}
	ticker := time.NewTicker(maintenance.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := renewUntilAvailable(ctx, maintenance, initial, maximum); err != nil {
				return err
			}
		}
	}
}

func registerUntilAvailable(ctx context.Context, maintenance ReplicaLeaseMaintenance, initial, maximum time.Duration) error {
	delay := initial
	for {
		if contextStopped(ctx) {
			return nil
		}
		request, err := registrationWithReadiness(maintenance)
		if err != nil {
			return err
		}
		_, err = maintenance.Client.Register(ctx, request)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrReplicaLifecycleUnavailable) {
			return err
		}
		if !waitForReplicaRetry(ctx, delay) {
			return nil
		}
		delay = nextReplicaRetry(delay, maximum)
	}
}

func renewUntilAvailable(ctx context.Context, maintenance ReplicaLeaseMaintenance, initial, maximum time.Duration) error {
	delay := initial
	for {
		if contextStopped(ctx) {
			return nil
		}
		readiness := maintenance.Readiness()
		if readiness.InstanceID != maintenance.Registration.Identity.InstanceID ||
			readiness.ReplicaID != maintenance.Registration.Identity.ReplicaID {
			return models.ErrReplicaIdentityMismatch
		}
		request := models.ReplicaRenewalRequest{
			ContractVersion:   maintenance.Registration.ContractVersion,
			Identity:          maintenance.Registration.Identity,
			AppliedGeneration: readiness.Generation,
			Ready:             readiness.Ready,
		}
		if err := request.Validate(); err != nil {
			return err
		}
		_, err := maintenance.Client.Renew(ctx, request)
		if err == nil {
			return nil
		}
		if errors.Is(err, models.ErrReplicaNotRegistered) {
			return registerUntilAvailable(ctx, maintenance, initial, maximum)
		}
		if !errors.Is(err, ErrReplicaLifecycleUnavailable) {
			return err
		}
		if !waitForReplicaRetry(ctx, delay) {
			return nil
		}
		delay = nextReplicaRetry(delay, maximum)
	}
}

func contextStopped(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func registrationWithReadiness(maintenance ReplicaLeaseMaintenance) (models.ReplicaRegistrationRequest, error) {
	readiness := maintenance.Readiness()
	if readiness.InstanceID != maintenance.Registration.Identity.InstanceID ||
		readiness.ReplicaID != maintenance.Registration.Identity.ReplicaID {
		return models.ReplicaRegistrationRequest{}, models.ErrReplicaIdentityMismatch
	}
	request := maintenance.Registration
	// Preserve the contract's required empty arrays; nil and [] are not
	// interchangeable in the versioned registration document.
	request.PeerEndpoints = append([]models.ReplicaPeerEndpoint{}, maintenance.Registration.PeerEndpoints...)
	request.AdvertisedContracts = append([]models.ContractVersion{}, maintenance.Registration.AdvertisedContracts...)
	request.AcceptedContracts = append([]models.ContractRange{}, maintenance.Registration.AcceptedContracts...)
	request.AppliedGeneration = readiness.Generation
	request.Ready = readiness.Ready
	if err := request.Validate(); err != nil {
		return models.ReplicaRegistrationRequest{}, err
	}
	return request, nil
}

func waitForReplicaRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextReplicaRetry(delay, maximum time.Duration) time.Duration {
	doubled := delay * 2
	if doubled < delay || doubled > maximum {
		return maximum
	}
	return doubled
}
