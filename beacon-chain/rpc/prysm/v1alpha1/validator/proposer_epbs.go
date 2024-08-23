package validator

import (
	"context"

	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/encoding/ssz"
	enginev1 "github.com/prysmaticlabs/prysm/v5/proto/engine/v1"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/time/slots"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetLocalHeader returns the local header for a given slot and proposer index.
func (vs *Server) GetLocalHeader(ctx context.Context, req *ethpb.HeaderRequest) (*enginev1.ExecutionPayloadHeaderEPBS, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.GetLocalHeader")
	defer span.End()

	if vs.SyncChecker.Syncing() {
		return nil, status.Error(codes.FailedPrecondition, "Syncing to latest head, not ready to respond")
	}

	if err := vs.optimisticStatus(ctx); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Validator is not ready to propose: %v", err)
	}

	slot := req.Slot
	epoch := slots.ToEpoch(slot)
	if params.BeaconConfig().EPBSForkEpoch > epoch {
		return nil, status.Errorf(codes.FailedPrecondition, "EPBS fork has not occurred yet")
	}

	st, parentRoot, err := vs.getParentState(ctx, slot)
	if err != nil {
		return nil, err
	}

	proposerIndex := req.ProposerIndex
	localPayload, err := vs.getLocalPayloadFromEngine(ctx, st, parentRoot, slot, proposerIndex)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get local payload: %v", err)
	}

	kzgRoot, err := ssz.KzgCommitmentsRoot(localPayload.BlobsBundle.KzgCommitments)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get kzg commitments root: %v", err)
	}

	return &enginev1.ExecutionPayloadHeaderEPBS{
		ParentBlockHash:        localPayload.ExecutionData.ParentHash(),
		ParentBlockRoot:        parentRoot[:],
		BlockHash:              localPayload.ExecutionData.BlockHash(),
		GasLimit:               localPayload.ExecutionData.GasLimit(),
		BuilderIndex:           proposerIndex,
		Slot:                   slot,
		Value:                  0,
		BlobKzgCommitmentsRoot: kzgRoot[:],
	}, nil
}
