#!/usr/bin/env node
/**
 * Local VeilVM economy loop: VAI mint/burn, AMM add/swap/remove, fee router, COL tranche.
 * Requires :9660 + :9098. Not Fuji. Does not claim a 13-of-20 committee.
 */
import { createHash, randomBytes } from "node:crypto";

const ROUTER = process.env.ORDER_ROUTER_URL || "http://127.0.0.1:9098";
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || "local-dev-secret";
const API =
  process.env.VEIL_API ||
  "http://127.0.0.1:9660/ext/bc/bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL/veilapi";

async function call(path, body) {
  const res = await fetch(`${ROUTER}${path}`, {
    method: "POST",
    headers: { "content-type": "application/json", "x-relay-secret": SECRET },
    body: JSON.stringify(body),
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok || json.accepted === false) {
    throw new Error(`${path} ${res.status}: ${JSON.stringify(json)}`);
  }
  return json;
}

async function rpc(method, params = {}) {
  const res = await fetch(API, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const json = await res.json();
  if (json.error) throw new Error(`${method}: ${json.error.message}`);
  return json.result;
}

function envelope() {
  const env = randomBytes(128);
  return {
    envelope: "0x" + env.toString("hex"),
    commitment: "0x" + createHash("sha256").update(env).digest("hex"),
    nullifier: "0x" + randomBytes(32).toString("hex"),
  };
}

const health = await (await fetch(`${ROUTER}/health`)).json();
if (!health.ok) throw new Error(`router: ${JSON.stringify(health)}`);
console.log(`1) router chain=${health.chainId}`);

const minted = await call("/native/mint-vai", { amount: 500 });
console.log(`2) mint ${minted.veilTxHash}`);

const burned = await call("/native/burn-vai", { amount: 50 });
console.log(`3) burn ${burned.veilTxHash}`);

const add = envelope();
const added = await call("/intents/native/liquidity/execute", {
  ...add,
  operation: "add_liquidity",
  asset0: 0,
  asset1: 1,
  amount0: 50,
  amount1: 50,
  minLP: 1,
});
console.log(`4) add_liquidity ${added.veilTxHash}`);

const sw = envelope();
const swapped = await call("/intents/native/liquidity/execute", {
  ...sw,
  operation: "swap_exact_in",
  assetIn: 0,
  assetOut: 1,
  amountIn: 5,
  minAmountOut: 1,
});
console.log(`5) swap ${swapped.veilTxHash}`);

const rm = envelope();
const removed = await call("/intents/native/liquidity/execute", {
  ...rm,
  operation: "remove_liquidity",
  asset0: 0,
  asset1: 1,
  lpAmount: 5,
  minAmount0: 1,
  minAmount1: 1,
});
console.log(`6) remove_liquidity ${removed.veilTxHash}`);

const fees = await call("/native/route-fees", { amount: 100 });
console.log(`7) route_fees ${fees.veilTxHash}`);

const col = await call("/native/release-col", { amount: 1000 });
console.log(`8) release_col ${col.veilTxHash}`);

const vai = await rpc("veilvm.vaistate");
const pool = await rpc("veilvm.pool", { asset0: 0, asset1: 1 });
const feeState = await rpc("veilvm.feeRouter");
const treasury = await rpc("veilvm.treasury");
console.log(
  JSON.stringify(
    {
      vaiDebt: vai.total_debt,
      pool,
      feeState,
      treasuryReleased: treasury.released,
      treasuryLocked: treasury.locked,
    },
    null,
    2,
  ),
);
console.log("ECOSYSTEM-LOOP PASS");
