package keeper_test

import (
	"fmt"
	"math"
	"math/big"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tharsis/ethermint/tests"
)

func (suite *KeeperTestSuite) TestGetHashFn() {
	header := suite.ctx.BlockHeader()
	h, _ := tmtypes.HeaderFromProto(&header)
	hash := h.Hash()

	testCases := []struct {
		msg      string
		height   uint64
		malleate func()
		expHash  common.Hash
	}{
		{
			"case 1.1: context hash cached",
			uint64(suite.ctx.BlockHeight()),
			func() {
				suite.ctx = suite.ctx.WithHeaderHash(tmhash.Sum([]byte("header")))
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			common.BytesToHash(tmhash.Sum([]byte("header"))),
		},
		{
			"case 1.2: failed to cast Tendermint header",
			uint64(suite.ctx.BlockHeight()),
			func() {
				header := tmproto.Header{}
				header.Height = suite.ctx.BlockHeight()
				suite.ctx = suite.ctx.WithBlockHeader(header)
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			common.Hash{},
		},
		{
			"case 1.3: hash calculated from Tendermint header",
			uint64(suite.ctx.BlockHeight()),
			func() {
				suite.ctx = suite.ctx.WithBlockHeader(header)
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			common.BytesToHash(hash),
		},
		{
			"case 2.1: height lower than current one, hist info not found",
			1,
			func() {
				suite.ctx = suite.ctx.WithBlockHeight(10)
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			common.Hash{},
		},
		{
			"case 2.2: height lower than current one, invalid hist info header",
			1,
			func() {
				suite.app.StakingKeeper.SetHistoricalInfo(suite.ctx, 1, &stakingtypes.HistoricalInfo{})
				suite.ctx = suite.ctx.WithBlockHeight(10)
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			common.Hash{},
		},
		{
			"case 2.3: height lower than current one, calculated from hist info header",
			1,
			func() {
				histInfo := &stakingtypes.HistoricalInfo{
					Header: header,
				}
				suite.app.StakingKeeper.SetHistoricalInfo(suite.ctx, 1, histInfo)
				suite.ctx = suite.ctx.WithBlockHeight(10)
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			common.BytesToHash(hash),
		},
		{
			"case 3: height greater than current one",
			200,
			func() {},
			common.Hash{},
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()

			hash := suite.app.EvmKeeper.GetHashFn()(tc.height)
			suite.Require().Equal(tc.expHash, hash)
		})
	}
}

func (suite *KeeperTestSuite) TestGetCoinbaseAddress() {
	valOpAddr := tests.GenerateAddress()

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"validator not found",
			func() {
				header := suite.ctx.BlockHeader()
				header.ProposerAddress = []byte{}
				suite.ctx = suite.ctx.WithBlockHeader(header)
				suite.app.EvmKeeper.WithContext(suite.ctx)
			},
			false,
		},
		{
			"success",
			func() {
				valConsAddr, privkey := tests.NewAddrKey()

				pkAny, err := codectypes.NewAnyWithValue(privkey.PubKey())
				suite.Require().NoError(err)

				validator := stakingtypes.Validator{
					OperatorAddress: sdk.ValAddress(valOpAddr.Bytes()).String(),
					ConsensusPubkey: pkAny,
				}

				suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
				err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
				suite.Require().NoError(err)

				header := suite.ctx.BlockHeader()
				header.ProposerAddress = valConsAddr.Bytes()
				suite.ctx = suite.ctx.WithBlockHeader(header)

				_, found := suite.app.StakingKeeper.GetValidatorByConsAddr(suite.ctx, valConsAddr.Bytes())
				suite.Require().True(found)

				suite.app.EvmKeeper.WithContext(suite.ctx)
				suite.Require().NotEmpty(suite.ctx.BlockHeader().ProposerAddress)
			},
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest() // reset

			tc.malleate()

			coinbase, err := suite.app.EvmKeeper.GetCoinbaseAddress(suite.ctx)
			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(valOpAddr, coinbase)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestGetEthIntrinsicGas() {
	testCases := []struct {
		name               string
		data               []byte
		accessList         ethtypes.AccessList
		height             int64
		isContractCreation bool
		noError            bool
		expGas             uint64
	}{
		{
			"no data, no accesslist, not contract creation, not homestead, not istanbul",
			nil,
			nil,
			1,
			false,
			true,
			params.TxGas,
		},
		{
			"with one zero data, no accesslist, not contract creation, not homestead, not istanbul",
			[]byte{0},
			nil,
			1,
			false,
			true,
			params.TxGas + params.TxDataZeroGas*1,
		},
		{
			"with one non zero data, no accesslist, not contract creation, not homestead, not istanbul",
			[]byte{1},
			nil,
			1,
			true,
			true,
			params.TxGas + params.TxDataNonZeroGasFrontier*1,
		},
		// we are not able to test the ErrGasUintOverflow due to RAM limitation
		// {
		// 	"with big data size overflow",
		// 	make([]byte, 271300000000000000),
		// 	nil,
		// 	1,
		// 	false,
		// 	false,
		// 	0,
		// },
		{
			"no data, one accesslist, not contract creation, not homestead, not istanbul",
			nil,
			[]ethtypes.AccessTuple{
				{},
			},
			1,
			false,
			true,
			params.TxGas + params.TxAccessListAddressGas,
		},
		{
			"no data, one accesslist with one storageKey, not contract creation, not homestead, not istanbul",
			nil,
			[]ethtypes.AccessTuple{
				{StorageKeys: make([]common.Hash, 1)},
			},
			1,
			false,
			true,
			params.TxGas + params.TxAccessListAddressGas + params.TxAccessListStorageKeyGas*1,
		},
		{
			"no data, no accesslist, is contract creation, is homestead, not istanbul",
			nil,
			nil,
			2,
			true,
			true,
			params.TxGasContractCreation,
		},
		{
			"with one zero data, no accesslist, not contract creation, is homestead, is istanbul",
			[]byte{1},
			nil,
			3,
			false,
			true,
			params.TxGas + params.TxDataNonZeroGasEIP2028*1,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest() // reset

			params := suite.app.EvmKeeper.GetParams(suite.ctx)
			ethCfg := params.ChainConfig.EthereumConfig(suite.app.EvmKeeper.ChainID())
			ethCfg.HomesteadBlock = big.NewInt(2)
			ethCfg.IstanbulBlock = big.NewInt(3)
			signer := ethtypes.LatestSignerForChainID(suite.app.EvmKeeper.ChainID())

			suite.ctx = suite.ctx.WithBlockHeight(tc.height)
			suite.app.EvmKeeper.WithContext(suite.ctx)

			m, err := newNativeMessage(
				suite.app.EvmKeeper.GetNonce(suite.address),
				suite.ctx.BlockHeight(),
				suite.address,
				ethCfg,
				suite.signer,
				signer,
				ethtypes.AccessListTxType,
				tc.data,
				tc.accessList,
			)
			suite.Require().NoError(err)

			gas, err := suite.app.EvmKeeper.GetEthIntrinsicGas(m, ethCfg, tc.isContractCreation)
			if tc.noError {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

			suite.Require().Equal(tc.expGas, gas)
		})
	}
}

func (suite *KeeperTestSuite) TestGasToRefund() {
	testCases := []struct {
		name           string
		gasconsumed    uint64
		refundQuotient uint64
		expGasRefund   uint64
		expPanic       bool
	}{
		{
			"gas refund 5",
			5,
			1,
			5,
			false,
		},
		{
			"gas refund 10",
			10,
			1,
			10,
			false,
		},
		{
			"gas refund availableRefund",
			11,
			1,
			10,
			false,
		},
		{
			"gas refund quotient 0",
			11,
			0,
			0,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.mintFeeCollector = true
			suite.SetupTest() // reset
			suite.app.EvmKeeper.AddRefund(10)

			if tc.expPanic {
				panicF := func() {
					suite.app.EvmKeeper.GasToRefund(tc.gasconsumed, tc.refundQuotient)
				}
				suite.Require().Panics(panicF)
			} else {
				gr := suite.app.EvmKeeper.GasToRefund(tc.gasconsumed, tc.refundQuotient)
				suite.Require().Equal(tc.expGasRefund, gr)
			}
		})
	}
	suite.mintFeeCollector = false
}

func (suite *KeeperTestSuite) TestRefundGas() {
	testCases := []struct {
		name           string
		leftoverGas    uint64
		refundQuotient uint64
		noError        bool
		expGasRefund   uint64
	}{
		{
			"leftoverGas more than tx gas limit",
			params.TxGas + 1,
			params.RefundQuotient,
			false,
			params.TxGas + 1,
		},
		{
			"leftoverGas equal to tx gas limit, insufficient fee collector account",
			params.TxGas,
			params.RefundQuotient,
			false,
			params.TxGas,
		},
		{
			"leftoverGas less than to tx gas limit",
			params.TxGas - 1,
			params.RefundQuotient,
			true,
			params.TxGas - 1,
		},
		{
			"no leftoverGas, refund half used gas ",
			0,
			params.RefundQuotient,
			true,
			params.TxGas / params.RefundQuotient,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.mintFeeCollector = true
			suite.SetupTest() // reset

			keeperParams := suite.app.EvmKeeper.GetParams(suite.ctx)
			ethCfg := keeperParams.ChainConfig.EthereumConfig(suite.app.EvmKeeper.ChainID())
			signer := ethtypes.LatestSignerForChainID(suite.app.EvmKeeper.ChainID())

			m, err := newNativeMessage(
				suite.app.EvmKeeper.GetNonce(suite.address),
				suite.ctx.BlockHeight(),
				suite.address,
				ethCfg,
				suite.signer,
				signer,
				ethtypes.AccessListTxType,
				nil,
				nil,
			)
			suite.Require().NoError(err)

			suite.app.EvmKeeper.AddRefund(params.TxGas)

			gr, err := suite.app.EvmKeeper.RefundGas(m, tc.leftoverGas, tc.refundQuotient)
			if tc.noError {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

			suite.Require().Equal(tc.expGasRefund, gr)
		})
	}
	suite.mintFeeCollector = false
}

func (suite *KeeperTestSuite) TestResetGasMeterAndConsumeGas() {
	testCases := []struct {
		name        string
		gasConsumed uint64
		gasUsed     uint64
		expPanic    bool
	}{
		{
			"gas consumed 5, used 5",
			5,
			5,
			false,
		},
		{
			"gas consumed 5, used 10",
			5,
			10,
			false,
		},
		{
			"gas consumed 10, used 10",
			10,
			10,
			false,
		},
		{
			"gas consumed 11, used 10, NegativeGasConsumed panic",
			11,
			10,
			true,
		},
		{
			"gas consumed 1, used 10, overflow panic",
			1,
			math.MaxUint64,
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest() // reset

			panicF := func() {
				gm := sdk.NewGasMeter(10)
				gm.ConsumeGas(tc.gasConsumed, "")
				suite.ctx = suite.ctx.WithGasMeter(gm)
				suite.app.EvmKeeper.WithContext(suite.ctx)

				suite.app.EvmKeeper.ResetGasMeterAndConsumeGas(tc.gasUsed)
			}

			if tc.expPanic {
				suite.Require().Panics(panicF)
			} else {
				suite.Require().NotPanics(panicF)
			}
		})
	}
}
