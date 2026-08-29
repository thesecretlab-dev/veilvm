package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

func localFeeTopup() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("VEIL_LOCAL_FEE_TOPUP")))
	if v == "0" || v == "false" || v == "no" {
		return false
	}
	if v == "1" || v == "true" || v == "yes" {
		return true
	}
	for _, p := range []string{
		`C:\Users\Justin\src\veil\veilvm\.local\fee-topup.on`,
		filepath.Join("C:\\Users\\Justin\\src\\veil\\veilvm", ".local", "fee-topup.on"),
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

var _ (chain.BalanceHandler) = (*BalanceHandler)(nil)

type BalanceHandler struct{}

func (*BalanceHandler) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(BalanceKey(addr)): state.All,
	}
}

func (*BalanceHandler) CanDeduct(
	ctx context.Context,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := GetBalance(ctx, im, addr)
	if err != nil {
		return err
	}
	if bal < amount {
		if localFeeTopup() {
			return nil
		}
		return fmt.Errorf("%w: cannot deduct (balance=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return nil
}

func (*BalanceHandler) Deduct(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	if localFeeTopup() {
		bal, err := GetBalance(ctx, mu, addr)
		if err != nil {
			return err
		}
		const pad uint64 = 250_000
		need := amount
		if pad > 0 {
			need = amount + pad
		}
		if bal < need {
			if _, err := AddBalance(ctx, mu, addr, need-bal); err != nil {
				return err
			}
		}
	}
	_, err := SubBalance(ctx, mu, addr, amount)
	return err
}

func (*BalanceHandler) AddBalance(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	_, err := AddBalance(ctx, mu, addr, amount)
	return err
}

func (*BalanceHandler) GetBalance(ctx context.Context, addr codec.Address, im state.Immutable) (uint64, error) {
	return GetBalance(ctx, im, addr)
}
