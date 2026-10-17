package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

type lifecycleMaintainerStub struct {
	registerCalls int
	renewCalls    int
	registerErrs  []error
	renewErrs     []error
	cancel        context.CancelFunc
	registrations []models.ReplicaRegistrationRequest
}

func (stub *lifecycleMaintainerStub) Register(_ context.Context, request models.ReplicaRegistrationRequest) (models.ReplicaRegistrationResponse, error) {
	stub.registerCalls++
	stub.registrations = append(stub.registrations, request)
	if len(stub.registerErrs) > 0 {
		err := stub.registerErrs[0]
		stub.registerErrs = stub.registerErrs[1:]
		if err != nil {
			return models.ReplicaRegistrationResponse{}, err
		}
	}
	if stub.cancel != nil && stub.registerCalls >= 2 && stub.renewCalls == 0 {
		stub.cancel()
	}
	now := time.Now().UTC()
	return models.ReplicaRegistrationResponse{
		ContractVersion: request.ContractVersion, Identity: request.Identity,
		ServerTime: now, LeaseExpiresAt: now.Add(30 * time.Second),
	}, nil
}

func (stub *lifecycleMaintainerStub) Renew(_ context.Context, request models.ReplicaRenewalRequest) (models.ReplicaLease, error) {
	stub.renewCalls++
	if len(stub.renewErrs) > 0 {
		err := stub.renewErrs[0]
		stub.renewErrs = stub.renewErrs[1:]
		if err != nil {
			return models.ReplicaLease{}, err
		}
	}
	if stub.cancel != nil && stub.renewCalls >= 2 {
		stub.cancel()
	}
	now := time.Now().UTC()
	return models.ReplicaLease{
		ContractVersion: request.ContractVersion, Identity: request.Identity,
		ServerTime: now, LeaseExpiresAt: now.Add(30 * time.Second),
	}, nil
}

func TestMaintainReplicaLeaseReregistersAfterCoreLosesLiveReplicaDirectory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &lifecycleMaintainerStub{
		cancel:    cancel,
		renewErrs: []error{models.ErrReplicaNotRegistered, nil},
	}
	registration := maintainerRegistration(t)
	readiness := func() models.Readiness {
		return models.Readiness{InstanceID: "server", ReplicaID: "replica-a", Generation: "generation-7", Ready: true}
	}
	err := MaintainReplicaLease(ctx, ReplicaLeaseMaintenance{
		Client: client, Registration: registration, Readiness: readiness,
		RenewInterval: time.Millisecond, RetryInitial: time.Millisecond, RetryMaximum: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("MaintainReplicaLease() error = %v", err)
	}
	if client.registerCalls != 2 || client.renewCalls < 2 {
		t.Fatalf("calls = register %d, renew %d; want re-registration after missing replica", client.registerCalls, client.renewCalls)
	}
	recovered := client.registrations[1]
	if recovered.Identity != registration.Identity || recovered.RestEndpoint != registration.RestEndpoint ||
		recovered.Release != registration.Release || recovered.AppliedGeneration != "generation-7" || !recovered.Ready {
		t.Fatalf("re-registration did not preserve immutable metadata and current readiness: %+v", recovered)
	}
}

func TestMaintainReplicaLeaseRetriesOnlyTemporaryCoreUnavailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &lifecycleMaintainerStub{
		cancel:       cancel,
		registerErrs: []error{ErrReplicaLifecycleUnavailable, nil},
	}
	err := MaintainReplicaLease(ctx, ReplicaLeaseMaintenance{
		Client: client, Registration: maintainerRegistration(t),
		Readiness:     func() models.Readiness { return models.Readiness{InstanceID: "server", ReplicaID: "replica-a"} },
		RenewInterval: time.Hour, RetryInitial: time.Millisecond, RetryMaximum: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("MaintainReplicaLease() error = %v", err)
	}
	if client.registerCalls != 2 || client.renewCalls != 0 {
		t.Fatalf("calls = register %d, renew %d; want one bounded retry before the first lease", client.registerCalls, client.renewCalls)
	}
}

func TestMaintainReplicaLeaseDoesNotReincarnateExpiredOrRefusedReplica(t *testing.T) {
	for _, testCase := range []struct {
		name string
		err  error
	}{{"expired", models.ErrReplicaLeaseExpired}, {"refused", models.ErrReplicaIdentityConflict}} {
		t.Run(testCase.name, func(t *testing.T) {
			client := &lifecycleMaintainerStub{renewErrs: []error{testCase.err}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := MaintainReplicaLease(ctx, ReplicaLeaseMaintenance{
				Client: client, Registration: maintainerRegistration(t),
				Readiness:     func() models.Readiness { return models.Readiness{InstanceID: "server", ReplicaID: "replica-a"} },
				RenewInterval: time.Millisecond, RetryInitial: time.Millisecond, RetryMaximum: time.Millisecond,
			})
			if !errors.Is(err, testCase.err) {
				t.Fatalf("MaintainReplicaLease() error = %v, want %v", err, testCase.err)
			}
			if client.registerCalls != 1 || client.renewCalls != 1 {
				t.Fatalf("calls = register %d, renew %d; must not retry terminal lease errors", client.registerCalls, client.renewCalls)
			}
		})
	}
}

func maintainerRegistration(t *testing.T) models.ReplicaRegistrationRequest {
	t.Helper()
	identity := models.PeerReplicaID{InstanceID: "server", ReplicaID: "replica-a", IncarnationID: "incarnation-a", PlacementID: "local-a"}
	request := models.ReplicaRegistrationRequest{
		ContractVersion: "replica-lifecycle/v2", Identity: identity,
		RestEndpoint: "https://server.local:19081", PeerEndpoints: []models.ReplicaPeerEndpoint{},
		Release:             models.ReplicaRelease{Version: "1.0.0", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		AdvertisedContracts: []models.ContractVersion{}, AcceptedContracts: []models.ContractRange{},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("test registration is invalid: %v", err)
	}
	return request
}
