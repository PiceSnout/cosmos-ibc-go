package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/ibc-go/modules/apps/29-fee/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

// ReceiveFee implements the ReceiveFee gRPC method
func (q Keeper) Fees(c context.Context, req *types.QueryFeesRequest) (*types.QueryFeesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	ctx = ctx.WithBlockHeight(int64(req.QueryHeight))
	

	return &types.QueryFeesResponse{}, nil
}

// IncentivizedPackets implements the IncentivizedPackets gRPC method
func (q Keeper) IncentivizedPackets(c context.Context, req *types.QueryIncentivizedPacketsRequest) (*types.QueryIncentivizedPacketsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	// update ctx to use QueryHeight
	packets := []*types.IdentifiedPacketFee{}
	store := prefix.NewStore(ctx.KVStore(q.storeKey), types.KeyFeeInEscrow())

	_, err := query.Paginate(store, req.Pagination, func(_, value []byte) error {
		result := q.MustUnmarshalFee(value)
		packets = append(packets, &result)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &types.QueryIncentivizedPacketsResponse{
		IncentivizedPackets: packets,
	}, nil
}

// IncentivizedPacket implements the IncentivizedPacket gRPC method
func (q Keeper) IncentivizedPacket(c context.Context, req *types.QueryIncentivizedPacketRequest) (*types.QueryIncentivizedPacketResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(c)
	// update ctx to use QueryHeight
	fee, exists := q.GetFeeInEscrow(ctx, req.PacketId.ChannelId, req.PacketId.Sequence)
	if !exists {
		return nil, status.Error(codes.NotFound, "no fee exists for this packetID")
	}

	return &types.QueryIncentivizedPacketResponse{
		IncentivizedPacket: &fee,
	}, nil
}
