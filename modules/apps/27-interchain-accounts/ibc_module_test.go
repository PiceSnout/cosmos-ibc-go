package interchain_accounts_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/stretchr/testify/suite"
	"github.com/tendermint/tendermint/crypto"

	"github.com/cosmos/ibc-go/v2/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v2/modules/core/24-host"
	ibctesting "github.com/cosmos/ibc-go/v2/testing"
)

var (
	// TestAccAddress defines a resuable bech32 address for testing purposes
	// TODO: update crypto.AddressHash() when sdk uses address.Module()
	TestAccAddress = types.GenerateAddress(sdk.AccAddress(crypto.AddressHash([]byte(types.ModuleName))), TestPortID)
	// TestOwnerAddress defines a reusable bech32 address for testing purposes
	TestOwnerAddress = "cosmos17dtl0mjt3t77kpuhg2edqzjpszulwhgzuj9ljs"
	// TestPortID defines a resuable port identifier for testing purposes
	TestPortID = fmt.Sprintf("%s-0-0-%s", types.VersionPrefix, TestOwnerAddress)
	// TestVersion defines a resuable interchainaccounts version string for testing purposes
	TestVersion = types.NewAppVersion(types.VersionPrefix, TestAccAddress.String())
)

type InterchainAccountsTestSuite struct {
	suite.Suite

	coordinator *ibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA *ibctesting.TestChain
	chainB *ibctesting.TestChain
}

func TestICATestSuite(t *testing.T) {
	suite.Run(t, new(InterchainAccountsTestSuite))
}

func (suite *InterchainAccountsTestSuite) SetupTest() {
	suite.coordinator = ibctesting.NewCoordinator(suite.T(), 3)
	suite.chainA = suite.coordinator.GetChain(ibctesting.GetChainID(0))
	suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(1))
}

func NewICAPath(chainA, chainB *ibctesting.TestChain) *ibctesting.Path {
	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig.PortID = types.PortID
	path.EndpointB.ChannelConfig.PortID = types.PortID
	path.EndpointA.ChannelConfig.Order = channeltypes.ORDERED
	path.EndpointB.ChannelConfig.Order = channeltypes.ORDERED
	path.EndpointA.ChannelConfig.Version = types.VersionPrefix
	path.EndpointB.ChannelConfig.Version = TestVersion

	return path
}

func InitInterchainAccount(endpoint *ibctesting.Endpoint, owner string) error {
	portID, err := types.GeneratePortID(owner, endpoint.ConnectionID, endpoint.Counterparty.ConnectionID)
	if err != nil {
		return err
	}
	channelSequence := endpoint.Chain.App.GetIBCKeeper().ChannelKeeper.GetNextChannelSequence(endpoint.Chain.GetContext())

	if err := endpoint.Chain.GetSimApp().ICAKeeper.InitInterchainAccount(endpoint.Chain.GetContext(), endpoint.ConnectionID, endpoint.Counterparty.ConnectionID, owner); err != nil {
		return err
	}

	// commit state changes for proof verification
	endpoint.Chain.App.Commit()
	endpoint.Chain.NextBlock()

	// update port/channel ids
	endpoint.ChannelID = channeltypes.FormatChannelIdentifier(channelSequence)
	endpoint.ChannelConfig.PortID = portID
	return nil
}

// SetupICAPath invokes the InterchainAccounts entrypoint and subsequent channel handshake handlers
func SetupICAPath(path *ibctesting.Path, owner string) error {
	if err := InitInterchainAccount(path.EndpointA, owner); err != nil {
		return err
	}

	if err := path.EndpointB.ChanOpenTry(); err != nil {
		return err
	}

	if err := path.EndpointA.ChanOpenAck(); err != nil {
		return err
	}

	if err := path.EndpointB.ChanOpenConfirm(); err != nil {
		return err
	}

	return nil
}

func (suite *InterchainAccountsTestSuite) TestOnChanOpenInit() {
	var (
		channel *channeltypes.Channel
	)
	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success", func() {}, true,
		},
		{
			"ICA OnChanOpenInit fails - UNORDERED channel", func() {
				channel.Ordering = channeltypes.UNORDERED
			}, false,
		},
		{
			"ICA auth module callback fails", func() {
				mockModule := suite.chainA.GetSimApp().GetMockModule()
				mockModule.IBCApp.OnChanOpenInit = func(ctx sdk.Context, order channeltypes.Order, connectionHops []string,
					portID, channelID string, chanCap *capabilitytypes.Capability,
					counterparty channeltypes.Counterparty, version string,
				) error {
					return fmt.Errorf("mock ica auth fails")
				}

			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			path := NewICAPath(suite.chainA, suite.chainB)
			suite.coordinator.SetupConnections(path)

			// mock init interchain account
			portID, err := types.GeneratePortID(TestOwnerAddress, path.EndpointA.ConnectionID, path.EndpointB.ConnectionID)
			suite.Require().NoError(err)
			portCap := suite.chainA.GetSimApp().IBCKeeper.PortKeeper.BindPort(suite.chainA.GetContext(), portID)
			suite.chainA.GetSimApp().ICAKeeper.ClaimCapability(suite.chainA.GetContext(), portCap, host.PortPath(portID))
			path.EndpointA.ChannelConfig.PortID = portID

			// default values
			counterparty := channeltypes.NewCounterparty(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
			channel = &channeltypes.Channel{
				State:          channeltypes.INIT,
				Ordering:       channeltypes.ORDERED,
				Counterparty:   counterparty,
				ConnectionHops: []string{path.EndpointA.ConnectionID},
				Version:        types.VersionPrefix,
			}

			tc.malleate()

			module, _, err := suite.chainA.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainA.GetContext(), path.EndpointA.ChannelConfig.PortID)
			suite.Require().NoError(err)

			chanCap, err := suite.chainA.App.GetScopedIBCKeeper().NewCapability(suite.chainA.GetContext(), host.ChannelCapabilityPath(ibctesting.TransferPort, path.EndpointA.ChannelID))
			suite.Require().NoError(err)

			cbs, ok := suite.chainA.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			err = cbs.OnChanOpenInit(suite.chainA.GetContext(), channel.Ordering, channel.GetConnectionHops(),
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, chanCap, channel.Counterparty, channel.GetVersion(),
			)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

		})
	}

}

func (suite *InterchainAccountsTestSuite) TestOnChanOpenTry() {
	var (
		channel *channeltypes.Channel
	)
	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{

		{
			"success", func() {}, true,
		},
		{
			"success: ICA auth module callback returns error", func() {
				// mock module callback should not be called on host side
				mockModule := suite.chainB.GetSimApp().GetMockModule()
				mockModule.IBCApp.OnChanOpenTry = func(ctx sdk.Context, order channeltypes.Order, connectionHops []string,
					portID, channelID string, chanCap *capabilitytypes.Capability,
					counterparty channeltypes.Counterparty, version, counterpartyVersion string,
				) error {
					return fmt.Errorf("mock ica auth fails")
				}

			}, true,
		},
		{
			"ICA callback fails - invalid version", func() {
				channel.Version = types.VersionPrefix
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			path := NewICAPath(suite.chainA, suite.chainB)
			counterpartyVersion := TestVersion
			suite.coordinator.SetupConnections(path)

			err := InitInterchainAccount(path.EndpointA, TestOwnerAddress)
			suite.Require().NoError(err)

			// default values
			counterparty := channeltypes.NewCounterparty(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			channel = &channeltypes.Channel{
				State:          channeltypes.TRYOPEN,
				Ordering:       channeltypes.ORDERED,
				Counterparty:   counterparty,
				ConnectionHops: []string{path.EndpointB.ConnectionID},
				Version:        TestVersion,
			}

			tc.malleate()

			module, _, err := suite.chainB.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainB.GetContext(), path.EndpointB.ChannelConfig.PortID)
			suite.Require().NoError(err)

			chanCap, err := suite.chainB.App.GetScopedIBCKeeper().NewCapability(suite.chainB.GetContext(), host.ChannelCapabilityPath(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID))
			suite.Require().NoError(err)

			cbs, ok := suite.chainB.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			err = cbs.OnChanOpenTry(suite.chainB.GetContext(), channel.Ordering, channel.GetConnectionHops(),
				path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, chanCap, channel.Counterparty, channel.GetVersion(), counterpartyVersion,
			)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

		})
	}

}

func (suite *InterchainAccountsTestSuite) TestOnChanOpenAck() {
	var (
		counterpartyVersion string
	)
	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success", func() {}, true,
		},
		{
			"ICA OnChanOpenACK fails - invalid version", func() {
				counterpartyVersion = "invalid|version"
			}, false,
		},
		{
			"ICA auth module callback fails", func() {
				mockModule := suite.chainA.GetSimApp().GetMockModule()
				mockModule.IBCApp.OnChanOpenAck = func(
					ctx sdk.Context, portID, channelID string, counterpartyVersion string,
				) error {
					return fmt.Errorf("mock ica auth fails")
				}
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {

			suite.SetupTest() // reset
			path := NewICAPath(suite.chainA, suite.chainB)
			counterpartyVersion = types.VersionPrefix
			suite.coordinator.SetupConnections(path)

			err := InitInterchainAccount(path.EndpointA, TestOwnerAddress)
			suite.Require().NoError(err)

			err = path.EndpointB.ChanOpenTry()
			suite.Require().NoError(err)

			tc.malleate()

			module, _, err := suite.chainA.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainA.GetContext(), types.PortID)
			suite.Require().NoError(err)

			cbs, ok := suite.chainA.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			err = cbs.OnChanOpenAck(suite.chainA.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, counterpartyVersion)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

		})
	}

}

func (suite *InterchainAccountsTestSuite) TestOnChanOpenConfirm() {
	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{

		{
			"success", func() {}, true,
		},
		{
			"success: ICA auth module callback returns error", func() {
				// mock module callback should not be called on host side
				mockModule := suite.chainB.GetSimApp().GetMockModule()
				mockModule.IBCApp.OnChanOpenConfirm = func(
					ctx sdk.Context, portID, channelID string,
				) error {
					return fmt.Errorf("mock ica auth fails")
				}

			}, true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest()
			path := NewICAPath(suite.chainA, suite.chainB)
			suite.coordinator.SetupConnections(path)

			err := InitInterchainAccount(path.EndpointA, TestOwnerAddress)
			suite.Require().NoError(err)

			err = path.EndpointB.ChanOpenTry()
			suite.Require().NoError(err)

			err = path.EndpointA.ChanOpenAck()
			suite.Require().NoError(err)

			tc.malleate()

			module, _, err := suite.chainB.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainB.GetContext(), path.EndpointB.ChannelConfig.PortID)
			suite.Require().NoError(err)

			cbs, ok := suite.chainB.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			err = cbs.OnChanOpenConfirm(suite.chainB.GetContext(), path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

		})
	}

}

func (suite *InterchainAccountsTestSuite) TestNegotiateAppVersion() {
	suite.SetupTest()
	path := NewICAPath(suite.chainA, suite.chainB)
	suite.coordinator.SetupConnections(path)

	module, _, err := suite.chainA.GetSimApp().GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainA.GetContext(), types.PortID)
	suite.Require().NoError(err)

	cbs, ok := suite.chainA.GetSimApp().GetIBCKeeper().Router.GetRoute(module)
	suite.Require().True(ok)

	counterpartyPortID, err := types.GeneratePortID(TestOwnerAddress, path.EndpointA.ConnectionID, path.EndpointB.ConnectionID)
	suite.Require().NoError(err)

	counterparty := &channeltypes.Counterparty{
		PortId:    counterpartyPortID,
		ChannelId: path.EndpointB.ChannelID,
	}

	version, err := cbs.NegotiateAppVersion(suite.chainA.GetContext(), channeltypes.ORDERED, path.EndpointA.ConnectionID, types.PortID, *counterparty, types.VersionPrefix)
	suite.Require().NoError(err)
	suite.Require().NoError(types.ValidateVersion(version))
	suite.Require().Equal(TestVersion, version)
}
