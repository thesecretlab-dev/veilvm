#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = dirname(fileURLToPath(import.meta.url));

function parseArgs(argv) {
  const args = {
    nodeUrl: process.env.NODE_URL || 'http://127.0.0.1:9660',
    chainId: process.env.CHAIN_ID || '',
    runSetup: false,
    pollSeconds: Number(process.env.SMOKE_POLL_SECONDS || 2),
    maxWaitSeconds: Number(process.env.SMOKE_MAX_WAIT_SECONDS || 90),
  };

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--node-url' && argv[i + 1]) {
      args.nodeUrl = argv[++i];
      continue;
    }
    if (a === '--chain-id' && argv[i + 1]) {
      args.chainId = argv[++i];
      continue;
    }
    if (a === '--poll-seconds' && argv[i + 1]) {
      args.pollSeconds = Number(argv[++i]);
      continue;
    }
    if (a === '--run-setup') {
      args.runSetup = true;
      continue;
    }
    if (a === '--max-wait-seconds' && argv[i + 1]) {
      args.maxWaitSeconds = Number(argv[++i]);
      continue;
    }
    if (a === '--help' || a === '-h') {
      printHelp();
      process.exit(0);
    }
    throw new Error(`unknown arg: ${a}`);
  }

  if (!Number.isFinite(args.pollSeconds) || args.pollSeconds <= 0) {
    throw new Error(`invalid --poll-seconds value: ${args.pollSeconds}`);
  }
  if (!Number.isFinite(args.maxWaitSeconds) || args.maxWaitSeconds <= 0) {
    throw new Error(`invalid --max-wait-seconds value: ${args.maxWaitSeconds}`);
  }

  return args;
}

function printHelp() {
  console.log('VEIL local smoke test');
  console.log('');
  console.log('Usage:');
  console.log('  node smoke-local.mjs --chain-id <CHAIN_ID> [--node-url <URL>] [--poll-seconds <N>]');
  console.log('  node smoke-local.mjs --run-setup [--node-url <URL>] [--poll-seconds <N>]');
  console.log('');
  console.log('Notes:');
  console.log('- --run-setup executes setup-local.mjs first and extracts SubnetID/ChainID from its output.');
  console.log('- For fresh-volume validation, wipe volumes before running with --run-setup.');
}

async function sleep(ms) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, ms));
}

async function jsonRpc(url, method, params = {}) {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });

  if (!res.ok) {
    throw new Error(`${method}: HTTP ${res.status}`);
  }

  const json = await res.json();
  if (json.error) {
    throw new Error(`${method}: ${json.error.message}`);
  }
  return json.result;
}

async function waitForHealthy(nodeUrl) {
  for (let i = 0; i < 60; i++) {
    try {
      const res = await fetch(`${nodeUrl}/ext/health`);
      if (!res.ok) {
        await sleep(1000);
        continue;
      }
      const body = await res.json();
      if (body.healthy) {
        return body;
      }
    } catch {}
    await sleep(1000);
  }
  throw new Error('node never became healthy within 60s');
}

async function getHealth(nodeUrl) {
  const res = await fetch(`${nodeUrl}/ext/health`);
  if (!res.ok) {
    throw new Error(`/ext/health returned HTTP ${res.status}`);
  }
  return await res.json();
}

function getChainHeightFromHealth(health, chainId) {
  const check = health?.checks?.[chainId];
  if (!check) {
    throw new Error(`health check missing for chain ${chainId}`);
  }

  const height = Number(check?.message?.engine?.consensus?.lastAcceptedHeight);
  if (!Number.isFinite(height)) {
    throw new Error(`invalid chain height in health payload for ${chainId}`);
  }
  return height;
}

function getChainHealthTimestamp(health, chainId) {
  const ts = health?.checks?.[chainId]?.timestamp;
  if (typeof ts !== 'string' || ts.length === 0) {
    throw new Error(`health timestamp missing for chain ${chainId}`);
  }
  return ts;
}

async function assertChainListed(nodeUrl, chainId) {
  const result = await jsonRpc(`${nodeUrl}/ext/bc/P`, 'platform.getBlockchains', {});
  const found = (result.blockchains || []).some((bc) => bc.id === chainId);
  if (!found) {
    throw new Error(`chain ${chainId} not found in platform.getBlockchains`);
  }
}

async function assertVeilApiGenesis(nodeUrl, chainId) {
  const result = await jsonRpc(
    `${nodeUrl}/ext/bc/${chainId}/veilapi`,
    'veilvm.genesis',
    {},
  );
  if (!result?.genesis?.initialRules) {
    throw new Error('veilvm.genesis returned unexpected payload');
  }
}

function runSetupScript() {
  const setupPath = resolve(scriptDir, 'setup-local.mjs');
  const child = spawnSync(process.execPath, [setupPath], {
    cwd: scriptDir,
    env: process.env,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 16,
  });

  const output = `${child.stdout || ''}\n${child.stderr || ''}`;
  if (child.status !== 0) {
    throw new Error(`setup-local.mjs failed\n${output}`);
  }

  if (!output.includes('CreateSubnet committed')) {
    throw new Error('setup flow did not confirm CreateSubnet committed');
  }
  if (!output.includes('CreateBlockchain committed')) {
    throw new Error('setup flow did not confirm CreateBlockchain committed');
  }

  const subnetMatch = output.match(/Subnet ID:\s+([A-Za-z0-9]+)/);
  const chainMatch = output.match(/Chain ID:\s+([A-Za-z0-9]+)/);
  if (!chainMatch) {
    throw new Error(`could not parse Chain ID from setup output\n${output}`);
  }

  return {
    subnetId: subnetMatch ? subnetMatch[1] : '',
    chainId: chainMatch[1],
  };
}

async function main() {
  const args = parseArgs(process.argv.slice(2));

  let chainId = args.chainId;
  let subnetId = '';

  if (args.runSetup) {
    console.log('Running setup-local.mjs to validate create-subnet/create-chain flow...');
    const parsed = runSetupScript();
    chainId = parsed.chainId;
    subnetId = parsed.subnetId;
    console.log(`Setup succeeded. SubnetID=${subnetId || 'n/a'} ChainID=${chainId}`);
  }

  if (!chainId) {
    throw new Error('missing chain ID: pass --chain-id <CHAIN_ID> or use --run-setup');
  }

  console.log(`Using node: ${args.nodeUrl}`);
  console.log(`Using chain: ${chainId}`);

  await waitForHealthy(args.nodeUrl);
  console.log('PASS: node healthy');

  await assertChainListed(args.nodeUrl, chainId);
  console.log('PASS: chain listed in platform.getBlockchains');

  await assertVeilApiGenesis(args.nodeUrl, chainId);
  console.log('PASS: veilvm.genesis reachable');

  const healthA = await getHealth(args.nodeUrl);
  const h1 = getChainHeightFromHealth(healthA, chainId);
  const ts1 = getChainHealthTimestamp(healthA, chainId);

  let h2 = h1;
  let ts2 = ts1;
  const maxLoops = Math.ceil(args.maxWaitSeconds / args.pollSeconds);
  for (let i = 0; i < maxLoops; i++) {
    await sleep(args.pollSeconds * 1000);
    const healthB = await getHealth(args.nodeUrl);
    ts2 = getChainHealthTimestamp(healthB, chainId);
    h2 = getChainHeightFromHealth(healthB, chainId);
    if (ts2 !== ts1) {
      break;
    }
  }

  if (ts2 === ts1) {
    throw new Error(
      `chain health timestamp did not refresh within ${args.maxWaitSeconds}s`,
    );
  }

  if (h2 <= h1) {
    throw new Error(`chain height did not increase after health refresh (${h1} -> ${h2})`);
  }
  console.log(`PASS: chain height increased (${h1} -> ${h2})`);

  console.log('SMOKE TEST PASSED');
}

main().catch((err) => {
  console.error(`SMOKE TEST FAILED: ${err.message || err}`);
  process.exit(1);
});
