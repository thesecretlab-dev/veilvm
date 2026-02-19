package vm

import (
	"context"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	"github.com/ava-labs/hypersdk/examples/veilvm/consts"
	vgenesis "github.com/ava-labs/hypersdk/examples/veilvm/genesis"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/utils"
)

const balanceCheckInterval = 500 * time.Millisecond

type JSONRPCClient struct {
	requester *requester.EndpointRequester

	g           *vgenesis.Genesis
	ruleFactory chain.RuleFactory
}

func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, consts.Name)
	return &JSONRPCClient{
		requester: req,
	}
}

func (cli *JSONRPCClient) Genesis(ctx context.Context) (*vgenesis.Genesis, error) {
	if cli.g != nil {
		return cli.g, nil
	}

	resp := new(GenesisReply)
	err := cli.requester.SendRequest(
		ctx,
		"genesis",
		nil,
		resp,
	)
	if err != nil {
		return nil, err
	}
	cli.g = resp.Genesis
	return resp.Genesis, nil
}

func (cli *JSONRPCClient) Balance(ctx context.Context, addr codec.Address) (uint64, error) {
	resp := new(BalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"balance",
		&BalanceArgs{
			Address: addr,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *JSONRPCClient) Market(ctx context.Context, marketID ids.ID) (*MarketReply, error) {
	resp := new(MarketReply)
	err := cli.requester.SendRequest(
		ctx,
		"market",
		&MarketArgs{
			MarketID: marketID,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) Pool(ctx context.Context, asset0 uint8, asset1 uint8) (*PoolReply, error) {
	resp := new(PoolReply)
	err := cli.requester.SendRequest(
		ctx,
		"pool",
		&PoolArgs{
			Asset0: asset0,
			Asset1: asset1,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) LPBalance(ctx context.Context, asset0 uint8, asset1 uint8, addr codec.Address) (uint64, error) {
	resp := new(LPBalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"lpbalance",
		&LPBalanceArgs{
			Asset0:  asset0,
			Asset1:  asset1,
			Address: addr,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *JSONRPCClient) VAIBalance(ctx context.Context, addr codec.Address) (uint64, error) {
	resp := new(VAIBalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"vaibalance",
		&VAIBalanceArgs{
			Address: addr,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *JSONRPCClient) VAIState(ctx context.Context) (*VAIStateReply, error) {
	resp := new(VAIStateReply)
	err := cli.requester.SendRequest(
		ctx,
		"vaistate",
		nil,
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) Treasury(ctx context.Context) (*TreasuryReply, error) {
	resp := new(TreasuryReply)
	err := cli.requester.SendRequest(
		ctx,
		"treasury",
		nil,
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) FeeRouter(ctx context.Context) (*FeeRouterReply, error) {
	resp := new(FeeRouterReply)
	err := cli.requester.SendRequest(
		ctx,
		"feerouter",
		nil,
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) Risk(ctx context.Context) (*RiskReply, error) {
	resp := new(RiskReply)
	err := cli.requester.SendRequest(
		ctx,
		"risk",
		nil,
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) Reserve(ctx context.Context) (*ReserveReply, error) {
	resp := new(ReserveReply)
	err := cli.requester.SendRequest(
		ctx,
		"reserve",
		nil,
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) ProofConfig(ctx context.Context) (*ProofConfigReply, error) {
	resp := new(ProofConfigReply)
	err := cli.requester.SendRequest(
		ctx,
		"proofconfig",
		nil,
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) BatchProof(ctx context.Context, marketID ids.ID, windowID uint64) (*BatchProofReply, error) {
	resp := new(BatchProofReply)
	err := cli.requester.SendRequest(
		ctx,
		"batchproof",
		&BatchProofArgs{
			MarketID: marketID,
			WindowID: windowID,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) VellumProof(ctx context.Context, marketID ids.ID, windowID uint64) (*VellumProofReply, error) {
	resp := new(VellumProofReply)
	err := cli.requester.SendRequest(
		ctx,
		"vellumproof",
		&VellumProofArgs{
			MarketID: marketID,
			WindowID: windowID,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) Bloodsworn(ctx context.Context, addr codec.Address) (*BloodswornReply, error) {
	resp := new(BloodswornReply)
	err := cli.requester.SendRequest(
		ctx,
		"bloodsworn",
		&BloodswornArgs{Address: addr},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) Glyph(ctx context.Context, marketID ids.ID, windowID uint64) (*GlyphReply, error) {
	resp := new(GlyphReply)
	err := cli.requester.SendRequest(
		ctx,
		"glyph",
		&GlyphArgs{
			MarketID: marketID,
			WindowID: windowID,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) ClearInputsHash(
	ctx context.Context,
	marketID ids.ID,
	windowID uint64,
	clearPrice uint64,
	totalVolume uint64,
	fillsHash []byte,
) (*ClearInputsHashReply, error) {
	resp := new(ClearInputsHashReply)
	err := cli.requester.SendRequest(
		ctx,
		"clearinputshash",
		&ClearInputsHashArgs{
			MarketID:    marketID,
			WindowID:    windowID,
			ClearPrice:  clearPrice,
			TotalVolume: totalVolume,
			FillsHash:   fillsHash,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) ZKMetrics(ctx context.Context, limit uint32, includeWindows bool) (*actions.ZKMetricsSnapshot, error) {
	resp := new(actions.ZKMetricsSnapshot)
	err := cli.requester.SendRequest(
		ctx,
		"zkmetrics",
		&ZKMetricsArgs{
			Limit:          limit,
			IncludeWindows: includeWindows,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) ResetZKMetrics(ctx context.Context) error {
	resp := new(ZKMetricsResetReply)
	return cli.requester.SendRequest(
		ctx,
		"zkmetricsreset",
		nil,
		resp,
	)
}

func (cli *JSONRPCClient) RecordZKProverMetrics(
	ctx context.Context,
	marketID ids.ID,
	windowID uint64,
	batchSizeHint uint32,
	witnessBuildMs int64,
	proofGenerationMs int64,
) error {
	resp := new(RecordZKProverMetricsReply)
	return cli.requester.SendRequest(
		ctx,
		"recordzkprovermetrics",
		&RecordZKProverMetricsArgs{
			MarketID:          marketID,
			WindowID:          windowID,
			BatchSizeHint:     batchSizeHint,
			WitnessBuildMs:    witnessBuildMs,
			ProofGenerationMs: proofGenerationMs,
		},
		resp,
	)
}

func (cli *JSONRPCClient) WaitForBalance(
	ctx context.Context,
	addr codec.Address,
	min uint64,
) error {
	return jsonrpc.Wait(ctx, balanceCheckInterval, func(ctx context.Context) (bool, error) {
		balance, err := cli.Balance(ctx, addr)
		if err != nil {
			return false, err
		}
		shouldExit := balance >= min
		if !shouldExit {
			utils.Outf(
				"{{yellow}}waiting for %s balance: %s{{/}}\n",
				utils.FormatBalance(min),
				addr,
			)
		}
		return shouldExit, nil
	})
}

func (*JSONRPCClient) GetParser() chain.Parser {
	return chain.NewTxTypeParser(ActionParser, AuthParser)
}

func (cli *JSONRPCClient) GetRuleFactory(ctx context.Context) (chain.RuleFactory, error) {
	if cli.ruleFactory != nil {
		return cli.ruleFactory, nil
	}
	networkGenesis, err := cli.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	cli.ruleFactory = &genesis.ImmutableRuleFactory{Rules: networkGenesis.Rules}
	return cli.ruleFactory, nil
}
