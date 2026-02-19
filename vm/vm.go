package vm

import (
	"errors"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	vgenesis "github.com/ava-labs/hypersdk/examples/veilvm/genesis"
	"github.com/ava-labs/hypersdk/examples/veilvm/storage"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]

	AuthProvider *auth.AuthProvider

	Parser *chain.TxTypeParser
)

func init() {
	ActionParser = codec.NewTypeParser[chain.Action]()
	AuthParser = codec.NewTypeParser[chain.Auth]()
	OutputParser = codec.NewTypeParser[codec.Typed]()
	AuthProvider = auth.NewAuthProvider()

	if err := auth.WithDefaultPrivateKeyFactories(AuthProvider); err != nil {
		panic(err)
	}

	if err := errors.Join(
		ActionParser.Register(&actions.Transfer{}, actions.UnmarshalTransfer),
		ActionParser.Register(&actions.CreateMarket{}, actions.UnmarshalCreateMarket),
		ActionParser.Register(&actions.CommitOrder{}, actions.UnmarshalCommitOrder),
		ActionParser.Register(&actions.RevealBatch{}, actions.UnmarshalRevealBatch),
		ActionParser.Register(&actions.ClearBatch{}, actions.UnmarshalClearBatch),
		ActionParser.Register(&actions.ResolveMarket{}, actions.UnmarshalResolveMarket),
		ActionParser.Register(&actions.Dispute{}, actions.UnmarshalDispute),
		ActionParser.Register(&actions.RouteFees{}, actions.UnmarshalRouteFees),
		ActionParser.Register(&actions.ReleaseCOLTranche{}, actions.UnmarshalReleaseCOLTranche),
		ActionParser.Register(&actions.MintVAI{}, actions.UnmarshalMintVAI),
		ActionParser.Register(&actions.BurnVAI{}, actions.UnmarshalBurnVAI),
		ActionParser.Register(&actions.CreatePool{}, actions.UnmarshalCreatePool),
		ActionParser.Register(&actions.AddLiquidity{}, actions.UnmarshalAddLiquidity),
		ActionParser.Register(&actions.RemoveLiquidity{}, actions.UnmarshalRemoveLiquidity),
		ActionParser.Register(&actions.SwapExactIn{}, actions.UnmarshalSwapExactIn),
		ActionParser.Register(&actions.UpdateReserveState{}, actions.UnmarshalUpdateReserveState),
		ActionParser.Register(&actions.SetRiskParams{}, actions.UnmarshalSetRiskParams),
		ActionParser.Register(&actions.SubmitBatchProof{}, actions.UnmarshalSubmitBatchProof),
		ActionParser.Register(&actions.SetProofConfig{}, actions.UnmarshalSetProofConfig),

		AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		OutputParser.Register(&actions.TransferResult{}, actions.UnmarshalTransferResult),
		OutputParser.Register(&actions.CreateMarketResult{}, actions.UnmarshalCreateMarketResult),
		OutputParser.Register(&actions.CommitOrderResult{}, actions.UnmarshalCommitOrderResult),
		OutputParser.Register(&actions.RevealBatchResult{}, actions.UnmarshalRevealBatchResult),
		OutputParser.Register(&actions.ClearBatchResult{}, actions.UnmarshalClearBatchResult),
		OutputParser.Register(&actions.ResolveMarketResult{}, actions.UnmarshalResolveMarketResult),
		OutputParser.Register(&actions.DisputeResult{}, actions.UnmarshalDisputeResult),
		OutputParser.Register(&actions.RouteFeesResult{}, actions.UnmarshalRouteFeesResult),
		OutputParser.Register(&actions.ReleaseCOLTrancheResult{}, actions.UnmarshalReleaseCOLTrancheResult),
		OutputParser.Register(&actions.MintVAIResult{}, actions.UnmarshalMintVAIResult),
		OutputParser.Register(&actions.BurnVAIResult{}, actions.UnmarshalBurnVAIResult),
		OutputParser.Register(&actions.CreatePoolResult{}, actions.UnmarshalCreatePoolResult),
		OutputParser.Register(&actions.AddLiquidityResult{}, actions.UnmarshalAddLiquidityResult),
		OutputParser.Register(&actions.RemoveLiquidityResult{}, actions.UnmarshalRemoveLiquidityResult),
		OutputParser.Register(&actions.SwapExactInResult{}, actions.UnmarshalSwapExactInResult),
		OutputParser.Register(&actions.UpdateReserveStateResult{}, actions.UnmarshalUpdateReserveStateResult),
		OutputParser.Register(&actions.SetRiskParamsResult{}, actions.UnmarshalSetRiskParamsResult),
		OutputParser.Register(&actions.SubmitBatchProofResult{}, actions.UnmarshalSubmitBatchProofResult),
		OutputParser.Register(&actions.SetProofConfigResult{}, actions.UnmarshalSetProofConfigResult),
	); err != nil {
		panic(err)
	}

	Parser = chain.NewTxTypeParser(ActionParser, AuthParser)
}

func New(options ...vm.Option) (*vm.VM, error) {
	factory := NewFactory()
	return factory.New(options...)
}

func NewFactory() *vm.Factory {
	options := append(defaultvm.NewDefaultOptions(), With())
	return vm.NewFactory(
		&vgenesis.Factory{},
		&storage.BalanceHandler{},
		metadata.NewDefaultManager(),
		ActionParser,
		AuthParser,
		OutputParser,
		auth.DefaultEngines(),
		options...,
	)
}
