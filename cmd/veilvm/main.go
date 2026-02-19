package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/ulimit"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/veilvm/cmd/veilvm/version"
	"github.com/ava-labs/hypersdk/snow"

	vvm "github.com/ava-labs/hypersdk/examples/veilvm/vm"
)

var rootCmd = &cobra.Command{
	Use:        "veilvm",
	Short:      "VEIL Protocol VM agent",
	SuggestFor: []string{"veilvm"},
	RunE:       runFunc,
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "veilvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
		return fmt.Errorf("%w: failed to set fd limit correctly", err)
	}

	v, err := vvm.New()
	if err != nil {
		return err
	}

	return rpcchainvm.Serve(context.TODO(), snow.NewSnowVM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]("v0.0.1", v))
}
