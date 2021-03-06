package workloads

import (
	"context"

	"github.com/oam-dev/kubevela/apis/standard.oam.dev/v1alpha1"
)

// WorkloadController is the interface that all type of cloneSet controller implements
type WorkloadController interface {
	// Size returns the total number of pods in the resources according to the spec
	Size(ctx context.Context) (int32, error)

	// VerifySpec makes sure that the resources can be upgraded according to the rollout plan
	// it returns new rollout status
	VerifySpec(ctx context.Context) (*v1alpha1.RolloutStatus, error)

	// Initialize make sure that the resource is ready to be upgraded
	// this function is tasked to change rollout status
	Initialize(ctx context.Context) (*v1alpha1.RolloutStatus, error)

	// RolloutOneBatchPods tries to upgrade pods in the resources following the rollout plan
	// it will upgrade as many pods as the rollout plan allows at once, the routine does not block on any operations.
	// Instead, we rely on the go-client's requeue mechanism to drive this towards the spec goal
	// it returns the number of pods upgraded in this round
	RolloutOneBatchPods(ctx context.Context) (*v1alpha1.RolloutStatus, error)

	// CheckOneBatchPods checks how many pods are ready to serve requests in the current batch
	// it returns whether the number of pods upgraded in this round satisfies the rollout plan
	CheckOneBatchPods(ctx context.Context) (*v1alpha1.RolloutStatus, bool)

	// FinalizeOneBatch makes sure that the rollout can start the next batch
	// it also needs to handle the corner cases around the very last batch
	FinalizeOneBatch(ctx context.Context) (*v1alpha1.RolloutStatus, error)

	// Finalize makes sure the resources are in a good final state.
	// It might depend on if the rollout succeeded or not.
	// For example, we may remove the source object to prevent scalar traits to ever work
	// and the finalize rollout web hooks will be called after this call succeeds
	Finalize(ctx context.Context, succeed bool) (*v1alpha1.RolloutStatus, error)
}
