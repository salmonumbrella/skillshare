// Independent always() cleanup also handles an interrupted capture process.
import assert from 'node:assert/strict';
import { readFile, readdir, readlink, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

assert.equal(process.env.CI, 'true');
assert.ok(process.env.RUNNER_TEMP);
const root = path.join(process.env.RUNNER_TEMP, 'pr880-fixtures');
const results = [];
let failed = false;
async function record(directory, name) {
  try { return JSON.parse(await readFile(path.join(directory, name), 'utf8')); }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
}
async function startTime(pid) {
  return (await readFile(`/proc/${pid}/stat`, 'utf8')).split(') ').pop().split(' ')[19];
}
async function identity(server) {
  const started = await startTime(server.pid);
  assert.match(started, /^\d+$/);
  const argv = (await readFile(`/proc/${server.pid}/cmdline`, 'utf8')).split('\0');
  assert.equal(argv[0], server.binary);
  assert.equal(argv[1], 'ui');
  assert.equal(await readlink(`/proc/${server.pid}/exe`), server.binary);
  assert.equal(await readlink(`/proc/${server.pid}/cwd`), server.cwd);
  assert.equal(await startTime(server.pid), started);
  return started;
}
for (const group of ['desktop', 'narrow-de']) {
  const directory = path.join(root, group);
  try {
    const recorded = await record(directory, 'server.json');
    const server = recorded ?? await record(directory, 'launch.json');
    if (!server) {
      await rm(directory, { recursive: true, force: true });
      results.push(`${group}: no launch descriptor; backend never spawned`);
      continue;
    }
    assert.ok(['pr880-before', 'pr880-after'].some(name => server.binary === path.join(process.env.RUNNER_TEMP, name)));
    assert.equal(server.cwd, path.join(directory, 'home'));
    if (!recorded) {
      const matches = [];
      for (const entry of (await readdir('/proc')).filter(name => /^\d+$/.test(name))) {
        try {
          const argv = (await readFile(`/proc/${entry}/cmdline`, 'utf8')).split('\0');
          if (argv[0] !== server.binary || argv[1] !== 'ui') continue;
          if (await readlink(`/proc/${entry}/cwd`) !== server.cwd) continue;
          const candidate = { ...server, pid: Number(entry) };
          matches.push({ ...candidate, startTime: await identity(candidate) });
        } catch (error) { if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error; }
      }
      assert.ok(matches.length <= 1, 'Ambiguous backend ownership; fixture retained');
      if (!matches.length) {
        await rm(directory, { recursive: true, force: true });
        results.push(`${group}: launch recorded; no matching backend remains`);
        continue;
      }
      Object.assign(server, matches[0]);
    }
    assert.ok(Number.isSafeInteger(server.pid) && server.pid > 1);
    assert.match(server.startTime, /^\d+$/);
    try {
      // Never signal a reused PID belonging to a different process or directory.
      assert.equal(await identity(server), server.startTime);
      process.kill(server.pid, 'SIGTERM');
      let alive = true;
      for (let i = 0; i < 30; i++) {
        try { process.kill(server.pid, 0); } catch (error) { if (error.code !== 'ESRCH') throw error; alive = false; break; }
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      if (alive) {
        // Recheck identity before the final signal.
        assert.equal(await identity(server), server.startTime);
        process.kill(server.pid, 'SIGKILL');
        for (let i = 0; i < 30; i++) {
          try { process.kill(server.pid, 0); } catch (error) { if (error.code !== 'ESRCH') throw error; alive = false; break; }
          await new Promise(resolve => setTimeout(resolve, 100));
        }
        assert.ok(!alive, 'Backend exit not confirmed; fixture retained');
      }
      results.push(`${group}: stopped owned backend ${server.pid}`);
    } catch (error) {
      if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error;
      results.push(`${group}: backend already stopped`);
    }
    await rm(directory, { recursive: true, force: true });
  } catch (error) {
    failed = true;
    results.push(`${group}: cleanup refused: ${error.stack}`);
  }
}
if (!failed) await rm(root, { recursive: true, force: true });
await writeFile(path.join(process.env.GITHUB_WORKSPACE, 'evidence/hosted-cleanup.json'), JSON.stringify({ failed, results }, null, 2));
if (failed) process.exitCode = 1;
