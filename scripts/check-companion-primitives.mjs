import { readFileSync } from "fs";
import { resolve } from "path";

const inputPath = process.argv[2] || "scripts/companion-evm.addresses.json";
const fullPath = resolve(process.cwd(), inputPath);
const upgradePath = process.argv[3] ? resolve(process.cwd(), process.argv[3]) : null;

const requiredFields = [
  "network",
  "chainId",
  "rpcUrl",
  "tempAdminEoa",
  "bridgeRelayer1",
  "bridgeRelayer2",
  "opsKeeper1",
  "deployer1",
  "teleporterRegistry",
  "teleporterMessenger",
  "nativeMinter",
  "txAllowList",
  "contractDeployerAllowList",
  "wveil",
  "bridgeMinterContract",
];

const recommendedFields = [
  "feeConfigManager",
  "multicall3",
  "faucet",
];

function isEmpty(value) {
  if (value === null || value === undefined) return true;
  if (typeof value === "string") return value.trim() === "";
  return false;
}

function fail(message) {
  console.error(`ERROR: ${message}`);
  process.exit(1);
}

function normalizeAddress(v) {
  return String(v || "").trim().toLowerCase();
}

function assertAddressSet(name, expected, actual) {
  const exp = new Set(expected.map(normalizeAddress).filter(Boolean));
  const got = new Set(actual.map(normalizeAddress).filter(Boolean));
  const expSorted = [...exp].sort();
  const gotSorted = [...got].sort();
  if (expSorted.length !== gotSorted.length || expSorted.some((v, i) => v !== gotSorted[i])) {
    fail(`${name} mismatch. expected=[${expSorted.join(", ")}] actual=[${gotSorted.join(", ")}]`);
  }
}

let doc;
try {
  doc = JSON.parse(readFileSync(fullPath, "utf8"));
} catch (err) {
  fail(`Unable to read/parse ${fullPath}: ${err.message}`);
}

const missingRequired = requiredFields.filter((field) => isEmpty(doc[field]));
const missingRecommended = recommendedFields.filter((field) => isEmpty(doc[field]));

if (doc.create2Deployer) {
  const canonical = "0x4e59b44847b379578588920cA78FbF26c0B4956C".toLowerCase();
  if (String(doc.create2Deployer).toLowerCase() !== canonical) {
    fail("create2Deployer does not match canonical 0x4e59...B4956C");
  }
}

if (missingRequired.length > 0) {
  fail(`Missing required primitive fields: ${missingRequired.join(", ")}`);
}

// Policy: treasury funding key must never be present in tx/deployer allowlists.
if (!isEmpty(doc.treasuryFundingAddress)) {
  const forbidden = normalizeAddress(doc.treasuryFundingAddress);
  const roleAddrs = [
    normalizeAddress(doc.tempAdminEoa),
    normalizeAddress(doc.bridgeRelayer1),
    normalizeAddress(doc.bridgeRelayer2),
    normalizeAddress(doc.opsKeeper1),
    normalizeAddress(doc.deployer1),
  ];
  if (roleAddrs.includes(forbidden)) {
    fail("treasuryFundingAddress must not be reused as admin/relayer/keeper/deployer");
  }
}

if (upgradePath) {
  let upgrade;
  try {
    upgrade = JSON.parse(readFileSync(upgradePath, "utf8"));
  } catch (err) {
    fail(`Unable to read/parse upgrade json ${upgradePath}: ${err.message}`);
  }
  const upgrades = Array.isArray(upgrade.precompileUpgrades) ? upgrade.precompileUpgrades : [];
  const txAllow = upgrades.find((u) => u.txAllowListConfig)?.txAllowListConfig;
  const deployAllow = upgrades.find((u) => u.contractDeployerAllowListConfig)?.contractDeployerAllowListConfig;
  const minter = upgrades.find((u) => u.contractNativeMinterConfig)?.contractNativeMinterConfig;

  if (!txAllow || !deployAllow || !minter) {
    fail("upgrade json missing one or more precompile configs: txAllowList/contractDeployerAllowList/contractNativeMinter");
  }

  assertAddressSet("txAllowList.enabled", [
    doc.bridgeRelayer1,
    doc.bridgeRelayer2,
    doc.opsKeeper1,
  ], txAllow.enabled || txAllow.enabledAddresses || []);

  assertAddressSet("txAllowList.admins", [
    doc.tempAdminEoa,
  ], txAllow.admins || txAllow.adminAddresses || []);

  assertAddressSet("contractDeployerAllowList.enabled", [
    doc.deployer1,
  ], deployAllow.enabled || deployAllow.enabledAddresses || []);

  assertAddressSet("contractDeployerAllowList.admins", [
    doc.tempAdminEoa,
  ], deployAllow.admins || deployAllow.adminAddresses || []);

  assertAddressSet("contractNativeMinter.enabled", [
    doc.bridgeMinterContract,
  ], minter.enabled || minter.enabledAddresses || []);

  assertAddressSet("contractNativeMinter.admins", [
    doc.tempAdminEoa,
  ], minter.admins || minter.adminAddresses || []);
}

console.log("PASS: required companion EVM primitive fields are populated.");
if (missingRecommended.length > 0) {
  console.log(`WARN: missing recommended fields: ${missingRecommended.join(", ")}`);
}
