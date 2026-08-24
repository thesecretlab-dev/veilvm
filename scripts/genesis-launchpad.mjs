#!/usr/bin/env node
/**
 * D01: freeze genesis buckets. Fail if they do not sum to TOTAL_SUPPLY.
 * v1 buckets: circulating + COL locked + COL live.
 * Keep3r is not a bucket and not a reservation.
 */
import { readFileSync, mkdirSync, writeFileSync } from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const genesis = JSON.parse(readFileSync(resolve(root, "genesis.json"), "utf8"));

const TOTAL = 990_999_000;

const circulating = (genesis.customAllocation || []).reduce((s, a) => s + Number(a.balance), 0);
const colLocked = Number(genesis.tokenomics.colVaultLocked);
const colLive = Number(genesis.tokenomics.colVaultLive);
const totalSupply = Number(genesis.tokenomics.totalSupply);
const sum = circulating + colLocked + colLive;
const feeSum =
  Number(genesis.tokenomics.feeRouterMsrbBips) +
  Number(genesis.tokenomics.feeRouterColBips) +
  Number(genesis.tokenomics.feeRouterOpsBips);

const errors = [];
if (totalSupply !== TOTAL) errors.push(`tokenomics.totalSupply ${totalSupply} != ${TOTAL}`);
if (sum !== TOTAL) errors.push(`buckets sum ${sum} != ${TOTAL}`);
if (feeSum !== 10_000) errors.push(`fee router bips ${feeSum} != 10000`);

const report = {
  id: "D01",
  timestamp: new Date().toISOString(),
  totalSupply: TOTAL,
  buckets: {
    circulating,
    colVaultLocked: colLocked,
    colVaultLive: colLive,
  },
  bucketSum: sum,
  feeRouterBips: {
    msrb: genesis.tokenomics.feeRouterMsrbBips,
    col: genesis.tokenomics.feeRouterColBips,
    ops: genesis.tokenomics.feeRouterOpsBips,
    sum: feeSum,
  },
  keep3r: {
    allocated: false,
    bips: 0,
    amount: 0,
    note: "Dropped 2026-08-24. Not a v1 genesis bucket. VeilKeep3r.sol stays parked. Cadence is relayer + operator, not a keeper market.",
  },
  placeholderChainID: genesis.initialRules?.chainID,
  pass: errors.length === 0,
  errors,
};

const outDir = resolve(root, "evidence-bundles/launchpad-freeze");
mkdirSync(outDir, { recursive: true });
const stamp = new Date().toISOString().replace(/[-:T.Z]/g, "").slice(0, 14);
const outFile = resolve(outDir, `launchpad-freeze-${stamp}.json`);
writeFileSync(outFile, JSON.stringify(report, null, 2) + "\n");
writeFileSync(resolve(outDir, "latest.txt"), `launchpad-freeze-${stamp}.json\n`);
console.log(JSON.stringify(report, null, 2));
console.error(`wrote ${outFile}`);
if (!report.pass) process.exitCode = 1;
