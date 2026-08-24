#!/usr/bin/env node
/**
 * Local stack E2E (no Docker):
 *   VeilVM (already running) + anvil companion rails + order-router + opaque relay.
 *
 * Requires: anvil, forge, cast, order-router on :9098, VeilVM on :9660.
 */
import { createHash, randomBytes } from 'node:crypto';
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const contractsDir = resolve(scriptDir, '../../veil-contracts');
const ROUTER = process.env.ORDER_ROUTER_URL || 'http://127.0.0.1:9098';
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || 'local-dev-secret';
const EVM_RPC = process.env.EVM_RPC_URL || 'http://127.0.0.1:8545';
const PK =
  process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY ||
  '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
const CAST = process.env.CAST_BIN || 'cast';
const FORGE = process.env.FORGE_BIN || 'forge';

function run(bin, args, opts = {}) {
  const r = spawnSync(bin, args, {
    encoding: 'utf8',
    cwd: opts.cwd,
    env: { ...process.env, ...(opts.env || {}) },
  });
  if (r.status !== 0) {
    throw new Error(`${bin} ${args.join(' ')}\n${r.stderr || r.stdout}`);
  }
  return (r.stdout || '').trim();
}

async function post(path, body) {
  const res = await fetch(`${ROUTER}${path}`, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      'x-relay-secret': SECRET,
    },
    body: JSON.stringify(body),
  });
  const json = await res.json();
  if (!res.ok || json.error || json.accepted === false) {
    throw new Error(`${path}: ${json.error || JSON.stringify(json)}`);
  }
  return json;
}

function sha256(buf) {
  return createHash('sha256').update(buf).digest();
}

console.log('1) create VeilVM market via router');
const created = await post('/native/create-market', {
  question: 'local-stack e2e',
  outcomes: 2,
  creatorBond: 1,
});
const marketId = created.marketId;
console.log(`   marketId=${marketId} tx=${created.veilTxHash}`);

console.log('2) deploy rails on anvil');
run(FORGE, ['--version'], { cwd: contractsDir });
const owner = run(CAST, ['wallet', 'address', PK]);
const deploy = (contract) => {
  const out = run(
    FORGE,
    [
      'create',
      contract,
      '--rpc-url',
      EVM_RPC,
      '--private-key',
      PK,
      '--broadcast',
      '--constructor-args',
      owner,
      owner,
    ],
    { cwd: contractsDir, env: { FOUNDRY_PROFILE: 'rails' } },
  );
  const m = out.match(/Deployed to:\s+(0x[a-fA-F0-9]{40})/);
  if (!m) throw new Error(`deploy parse failed:\n${out}`);
  return m[1];
};
const orderGw = deploy('contracts/bridge/VeilOrderIntentGateway.sol:VeilOrderIntentGateway');
console.log(`   order gateway ${orderGw}`);

console.log('3) mailbox + submitIntent');
const envelope = randomBytes(128);
const commitment = '0x' + sha256(envelope).toString('hex');
const nullifier = '0x' + randomBytes(32).toString('hex');
const mailboxPath = resolve(scriptDir, 'intent-mailbox.json');
mkdirSync(scriptDir, { recursive: true });
writeFileSync(
  mailboxPath,
  JSON.stringify(
    {
      [commitment]: {
        envelope: '0x' + envelope.toString('hex'),
        marketKey: marketId,
        marketType: 'veil_native',
        routingFeeBps: 0,
        windowId: 1,
      },
    },
    null,
    2,
  ),
);
const submitOut = run(CAST, [
  'send',
  orderGw,
  'submitIntent(bytes32,bytes32)',
  commitment,
  nullifier,
  '--rpc-url',
  EVM_RPC,
  '--private-key',
  PK,
  '--json',
]);
let receipt;
try {
  receipt = JSON.parse(submitOut);
} catch {
  const hash = submitOut.match(/0x[a-fA-F0-9]{64}/)?.[0];
  if (!hash) throw new Error(`submitIntent output:\n${submitOut}`);
  receipt = JSON.parse(run(CAST, ['receipt', hash, '--rpc-url', EVM_RPC, '--json']));
}
const log0 = (receipt.logs || []).find((l) => l.address?.toLowerCase() === orderGw.toLowerCase());
if (!log0?.topics?.[1]) throw new Error(`no IntentSubmitted log: ${JSON.stringify(receipt)}`);
const intentId = log0.topics[1];
console.log(`   intentId=${intentId}`);

console.log('4) relay');
const relay = spawnSync(process.execPath, [resolve(scriptDir, 'relay-opaque-intents.mjs')], {
  encoding: 'utf8',
  env: {
    ...process.env,
    ORDER_ROUTER_URL: ROUTER,
    ORDER_ROUTER_RELAY_SECRET: SECRET,
    EVM_RPC_URL: EVM_RPC,
    ORDER_GATEWAY: orderGw,
    VEIL_INTENT_MAILBOX_PATH: mailboxPath,
    EVM_RELAY_EXECUTOR_PRIVATE_KEY: PK,
    CAST_BIN: CAST,
  },
});
process.stdout.write(relay.stdout || '');
process.stderr.write(relay.stderr || '');
if (relay.status !== 0) {
  throw new Error('relay-opaque-intents failed');
}

console.log('5) getIntent state (2 = EXECUTED)');
const stateOut = run(CAST, [
  'call',
  orderGw,
  'getIntent(bytes32)(bytes32,bytes32,bytes32,uint64,uint8)',
  intentId,
  '--rpc-url',
  EVM_RPC,
]);
console.log(`   ${stateOut}`);
if (!stateOut.includes('2')) {
  throw new Error(`expected EXECUTED (2), got: ${stateOut}`);
}

console.log('6) mint VAI for liquidity');
const minted = await post('/native/mint-vai', { amount: 10_000 });
console.log(`   mint tx=${minted.veilTxHash}`);

console.log('7) liquidity intent');
const liqGw = deploy('contracts/bridge/VeilLiquidityIntentGateway.sol:VeilLiquidityIntentGateway');
console.log(`   liq gateway ${liqGw}`);
const liqEnvelope = randomBytes(128);
const liqCommitment = '0x' + sha256(liqEnvelope).toString('hex');
const liqNullifier = '0x' + randomBytes(32).toString('hex');
const liqMailbox = resolve(scriptDir, 'intent-mailbox-liq.json');
writeFileSync(
  liqMailbox,
  JSON.stringify(
    {
      [liqCommitment]: {
        envelope: '0x' + liqEnvelope.toString('hex'),
        operation: 'add_liquidity',
        asset0: 0,
        asset1: 1,
        amount0: 1000,
        amount1: 1000,
        minLP: 1,
      },
    },
    null,
    2,
  ),
);
const liqSubmitOut = run(CAST, [
  'send',
  liqGw,
  'submitIntent(bytes32,bytes32)',
  liqCommitment,
  liqNullifier,
  '--rpc-url',
  EVM_RPC,
  '--private-key',
  PK,
  '--json',
]);
let liqReceipt;
try {
  liqReceipt = JSON.parse(liqSubmitOut);
} catch {
  const hash = liqSubmitOut.match(/0x[a-fA-F0-9]{64}/)?.[0];
  if (!hash) throw new Error(`liq submitIntent output:\n${liqSubmitOut}`);
  liqReceipt = JSON.parse(run(CAST, ['receipt', hash, '--rpc-url', EVM_RPC, '--json']));
}
const liqLog = (liqReceipt.logs || []).find((l) => l.address?.toLowerCase() === liqGw.toLowerCase());
if (!liqLog?.topics?.[1]) throw new Error(`no LiquidityIntentSubmitted log: ${JSON.stringify(liqReceipt)}`);
const liqIntentId = liqLog.topics[1];
const liqRelay = spawnSync(process.execPath, [resolve(scriptDir, 'relay-opaque-intents.mjs')], {
  encoding: 'utf8',
  env: {
    ...process.env,
    ORDER_ROUTER_URL: ROUTER,
    ORDER_ROUTER_RELAY_SECRET: SECRET,
    EVM_RPC_URL: EVM_RPC,
    LIQUIDITY_GATEWAY: liqGw,
    VEIL_INTENT_MAILBOX_PATH: liqMailbox,
    EVM_RELAY_EXECUTOR_PRIVATE_KEY: PK,
    CAST_BIN: CAST,
  },
});
process.stdout.write(liqRelay.stdout || '');
process.stderr.write(liqRelay.stderr || '');
if (liqRelay.status !== 0) {
  throw new Error('liquidity relay failed');
}
const liqStateOut = run(CAST, [
  'call',
  liqGw,
  'getIntent(bytes32)(bytes32,bytes32,bytes32,uint64,uint8)',
  liqIntentId,
  '--rpc-url',
  EVM_RPC,
]);
console.log(`   ${liqStateOut}`);
if (!liqStateOut.trim().split(/\s+/).includes('2')) {
  throw new Error(`expected liquidity EXECUTED (2), got: ${liqStateOut}`);
}

console.log('LOCAL STACK E2E PASSED');
console.log(
  JSON.stringify({ marketId, orderGw, liqGw, intentId, liqIntentId, commitment, nullifier }, null, 2),
);
