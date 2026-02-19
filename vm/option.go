package vm

import (
	"log"
	"os"
	"strings"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/examples/veilvm/actions"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	"github.com/ava-labs/hypersdk/examples/veilvm/zk"
	"github.com/ava-labs/hypersdk/vm"
)

const Namespace = "controller"
const defaultLocalGroth16VKPath = "/root/.avalanchego/zk/groth16_clearhash_vk.bin"

type Config struct {
	Enabled bool             `json:"enabled"`
	ZK      ZKVerifierConfig `json:"zk"`
}

type ZKVerifierConfig struct {
	Enabled bool `json:"enabled"`
	Strict  bool `json:"strict"`

	Groth16VerifyingKeyPath string `json:"groth16VerifyingKeyPath"`
	PlonkVerifyingKeyPath   string `json:"plonkVerifyingKeyPath"`
	RequiredCircuitID       string `json:"requiredCircuitID"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled: true,
		ZK:      NewDefaultZKVerifierConfig(),
	}
}

func NewDefaultZKVerifierConfig() ZKVerifierConfig {
	return ZKVerifierConfig{
		Enabled: false,
		Strict:  false,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), func(_ api.VM, config Config) (vm.Opt, error) {
		resolved := resolveZKConfig(config.ZK)
		log.Printf(
			"veilvm zk verifier config: enabled=%t strict=%t groth16_vk_set=%t plonk_vk_set=%t required_circuit_id=%q",
			resolved.Enabled,
			resolved.Strict,
			strings.TrimSpace(resolved.Groth16VerifyingKeyPath) != "",
			strings.TrimSpace(resolved.PlonkVerifyingKeyPath) != "",
			strings.TrimSpace(resolved.RequiredCircuitID),
		)
		if err := installBatchProofVerifier(resolved); err != nil {
			return vm.NewOpt(), err
		}

		if !config.Enabled {
			return vm.NewOpt(), nil
		}
		return vm.WithVMAPIs(jsonRPCServerFactory{}), nil
	})
}

func resolveZKConfig(cfg ZKVerifierConfig) ZKVerifierConfig {
	if v, ok := parseEnvBool("VEIL_ZK_VERIFIER_ENABLED"); ok {
		cfg.Enabled = v
	}
	if v, ok := parseEnvBool("VEIL_ZK_VERIFIER_STRICT"); ok {
		cfg.Strict = v
	}
	if v, ok := getEnv("VEIL_ZK_GROTH16_VK_PATH"); ok {
		cfg.Groth16VerifyingKeyPath = v
	}
	if v, ok := getEnv("VEIL_ZK_PLONK_VK_PATH"); ok {
		cfg.PlonkVerifyingKeyPath = v
	}
	if v, ok := getEnv("VEIL_ZK_REQUIRED_CIRCUIT_ID"); ok {
		cfg.RequiredCircuitID = v
	}
	// Local/dev fallback: if env did not flow into the VM subprocess but the
	// standard fixture VK exists in-container, fail closed by default.
	if !cfg.Enabled &&
		strings.TrimSpace(cfg.Groth16VerifyingKeyPath) == "" &&
		strings.TrimSpace(cfg.PlonkVerifyingKeyPath) == "" {
		if _, err := os.Stat(defaultLocalGroth16VKPath); err == nil {
			cfg.Enabled = true
			cfg.Strict = true
			cfg.Groth16VerifyingKeyPath = defaultLocalGroth16VKPath
			if strings.TrimSpace(cfg.RequiredCircuitID) == "" {
				cfg.RequiredCircuitID = mconsts.ProofCircuitClearHashV1
			}
		}
	}
	return cfg
}

func installBatchProofVerifier(cfg ZKVerifierConfig) error {
	if !cfg.Enabled {
		actions.ConfigureBatchProofVerifier(nil, cfg.Strict)
		return nil
	}

	verifier, err := zk.NewVerifier(zk.Config{
		Groth16VerifyingKeyPath: cfg.Groth16VerifyingKeyPath,
		PlonkVerifyingKeyPath:   cfg.PlonkVerifyingKeyPath,
		RequiredCircuitID:       cfg.RequiredCircuitID,
	})
	if err != nil {
		return err
	}
	actions.ConfigureBatchProofVerifier(verifier, cfg.Strict)
	return nil
}

func parseEnvBool(name string) (bool, bool) {
	v, ok := getEnv(name)
	if !ok {
		return false, false
	}
	switch strings.ToLower(v) {
	case "1", "true", "t", "yes", "y", "on":
		return true, true
	case "0", "false", "f", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func getEnv(name string) (string, bool) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return getParentProcessEnv(name)
	}
	return v, true
}

func getParentProcessEnv(name string) (string, bool) {
	blob, err := os.ReadFile("/proc/1/environ")
	if err != nil || len(blob) == 0 {
		return "", false
	}
	target := name + "="
	for _, entry := range strings.Split(string(blob), "\x00") {
		if strings.HasPrefix(entry, target) {
			v := strings.TrimSpace(strings.TrimPrefix(entry, target))
			if v == "" {
				return "", false
			}
			return v, true
		}
	}
	return "", false
}
