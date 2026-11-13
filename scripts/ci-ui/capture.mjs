// Authoring this harness is not evidence. Only an executed hosted run can certify it.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { command, makeFixture, pins } from './fixture.mjs';

assert.equal(process.env.CI, 'true', 'This helper is for hosted CI only');
assert.ok(process.env.RUNNER_TEMP);
const workspace = process.env.GITHUB_WORKSPACE;
const output = path.join(workspace, 'evidence');
const stateRoot = process.env.CAPTURE_STATE_ROOT;
assert.equal(stateRoot, path.join(process.env.RUNNER_TEMP, 'pr880-fixtures'));
const { chromium } = await import(pathToFileURL(process.env.CAPTURE_PLAYWRIGHT).href);
const viewport = { width: 1440, height: 1100 };
const narrowViewport = { width: 1024, height: 900 };
const names = ['_absent', '_occupied', '_blocked', '_restored', '_restored-dirty', '_restored-status', '_clean', '_dirty'];
const declarationStates = [
  'absent-row', 'occupied-row', 'absent-confirm', 'absent-result', 'occupied-confirm', 'occupied-result',
  'blocked-confirm', 'blocked-error',
  'restored-confirm', 'restored-result',
  'restored-dirty-confirm', 'restored-dirty-refusal', 'restored-dirty-result',
  'restored-status-confirm', 'restored-status-refusal', 'restored-status-result',
];
const manifest = {
  status: 'incomplete', pins, captures: [], checks: [], deleteRequests: [],
  provenance: 'Actual pinned dashboard builds and actual isolated backend; synthetic filesystem data. No native clients exercised.',
  syntheticInterceptions: ['_blocked DELETE: nonforceable 500', '_restored-status first DELETE: repo_status_failed 409 (current real checkout)'],
  media: 'Dark clean style, CSS viewport crops only. Browser devicePixelRatio measured; no physical display claim. Caption/pointer are capture overlays.',
};
let browser;
let activeFixture;
let activeContext;
let recorder;
await mkdir(output, { recursive: true });

function check(description) { manifest.checks.push(description); }
function translate(messages, key, params = {}) {
  assert.equal(typeof messages[key], 'string', `Missing source locale key ${key}`);
  return messages[key].replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`));
}
async function caption(page, text) {
  await page.evaluate(text => { document.querySelector('#ci-caption').textContent = text; }, text);
}
async function overlays(page) {
  // Only annotations are injected. No app DOM, API result or product assertion is replaced.
  await page.evaluate(() => {
    const banner = document.createElement('div');
    banner.id = 'ci-caption';
    Object.assign(banner.style, {
      position: 'fixed', inset: '0 0 auto 0', height: '52px', zIndex: '2147483646',
      background: '#15171b', color: '#fff', font: '600 21px/52px system-ui', textAlign: 'center', pointerEvents: 'none',
    });
    document.body.append(banner);
    const pointer = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    pointer.id = 'ci-pointer';
    pointer.setAttribute('width', '28'); pointer.setAttribute('height', '36');
    pointer.setAttribute('viewBox', '-2 -2 28 36');
    // macOS-style black arrow, white outline; its tip is at CSS (left, top).
    pointer.innerHTML = '<path d="M0 0 L0 25 L6 19 L11 31 L16 29 L11 17 L21 17 Z" fill="black" stroke="white" stroke-width="1.5" stroke-linejoin="round"/>';
    Object.assign(pointer.style, { position: 'fixed', left: '320px', top: '170px', zIndex: '2147483647', pointerEvents: 'none', overflow: 'visible', transform: 'translate(-2px,-2px)' });
    document.body.append(pointer);
  });
}

async function click(page, target) {
  await target.waitFor({ state: 'visible' });
  await target.scrollIntoViewIfNeeded();
  const box = await target.boundingBox();
  assert.ok(box);
  const tip = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  assert.ok(tip.y > 52, 'Click target must remain below caption');
  const from = await page.locator('#ci-pointer').evaluate(el => ({ x: parseFloat(el.style.left), y: parseFloat(el.style.top) }));
  for (let i = 1; i <= 24; i++) {
    const t = i / 24;
    const eased = t * t * (3 - 2 * t);
    const x = from.x + (tip.x - from.x) * eased;
    const y = from.y + (tip.y - from.y) * eased;
    await page.mouse.move(x, y);
    await page.locator('#ci-pointer').evaluate((el, { x, y }) => { el.style.left = `${x}px`; el.style.top = `${y}px`; }, { x, y });
    await page.waitForTimeout(28);
  }
  await page.mouse.click(tip.x, tip.y);
}

async function shot(page, side, state, title, group = 'desktop') {
  await caption(page, title);
  await page.evaluate(() => window.scrollTo(0, 0));
  assert.ok(await page.locator('html').evaluate(el => el.classList.contains('dark')));
  const metrics = await page.evaluate(() => ({
    viewport: { width: innerWidth, height: innerHeight }, devicePixelRatio,
    scroll: { x: scrollX, y: scrollY }, crop: { x: 0, y: 0, width: innerWidth, height: innerHeight },
    background: getComputedStyle(document.body).backgroundColor,
    regions: [...document.querySelectorAll('.ss-dlg, .ss-dlg .db, .ss-dlg .df, .ss-r:has(button[aria-expanded])')].map(el => {
      const r = el.getBoundingClientRect();
      return { text: el.textContent, x: r.x, y: r.y, width: r.width, height: r.height, scrollWidth: el.scrollWidth, clientWidth: el.clientWidth };
    }),
  }));
  const file = `${group}-${side}-${state}.png`;
  await page.screenshot({ path: path.join(output, file), fullPage: false });
  manifest.captures.push({ side, state, group, file, title, metrics });
}

async function startRecording(page) {
  const directory = path.join(output, 'recording-frames');
  await mkdir(directory, { recursive: true });
  let stop = false;
  let recordingError;
  const frames = [];
  const loop = (async () => {
    while (!stop) {
      assert.ok(await page.locator('html').evaluate(el => el.classList.contains('dark')));
      const name = `${String(frames.length).padStart(6, '0')}.jpg`;
      const time = performance.now();
      const title = await page.locator('#ci-caption').textContent();
      await page.screenshot({ path: path.join(directory, name), type: 'jpeg', quality: 90, fullPage: false });
      frames.push({ name, time, title });
      await page.waitForTimeout(100);
    }
  })().catch(error => { recordingError = error; stop = true; });
  return async () => {
    stop = true;
    await loop;
    if (recordingError) throw recordingError;
    assert.ok(frames.length > 1);
    // Preserve actual sample timing. The frames begin only after the loaded dark app.
    const list = frames.flatMap((frame, i) => [
      `file '${frame.name}'`,
      `duration ${i + 1 < frames.length ? (frames[i + 1].time - frame.time) / 1000 : 0.15}`,
    ]);
    list.push(`file '${frames.at(-1).name}'`);
    await writeFile(path.join(directory, 'frames.txt'), list.join('\n'));
    await writeFile(path.join(directory, 'frames.json'), JSON.stringify(frames, null, 2));
    command('ffmpeg', ['-y', '-f', 'concat', '-safe', '0', '-i', path.join(directory, 'frames.txt'), '-vf', 'fps=30', '-c:v', 'libx264', '-crf', '18', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', path.join(output, 'removal-sequence.mp4')]);
    const probe = JSON.parse(command('ffprobe', ['-v', 'error', '-show_streams', '-show_format', '-of', 'json', path.join(output, 'removal-sequence.mp4')]));
    assert.equal(probe.streams[0].codec_name, 'h264');
    assert.equal(probe.streams[0].width, viewport.width);
    assert.equal(probe.streams[0].height, viewport.height);
    manifest.recording = {
      file: 'removal-sequence.mp4', frames: frames.length, probe,
      captionFrames: frames.filter((frame, i) => i === 0 || frame.title !== frames[i - 1].title),
    };
  };
}

async function session(side, group, size, locale, fixtureNames = names) {
  const app = path.join(workspace, side);
  assert.equal(command('git', ['-C', app, 'rev-parse', 'HEAD']).trim(), pins[side]);
  // Reuse the SAME logical and absolute fixture paths after the previous side's cleanup.
  const fixture = await makeFixture(path.join(stateRoot, group), app, path.join(process.env.RUNNER_TEMP, `pr880-${side}`), fixtureNames);
  activeFixture = fixture;
  const url = await fixture.start(side === 'before' ? 19421 : 19422);
  const context = await browser.newContext({ viewport: size, deviceScaleFactor: 1, colorScheme: 'dark', locale: locale === 'de' ? 'de-DE' : 'en-US', serviceWorkers: 'block' });
  activeContext = context;
  await context.addInitScript(locale => {
    localStorage.setItem('skillshare-theme-preference', 'dark');
    localStorage.setItem('skillshare-style', 'clean');
    localStorage.setItem('skillshare:locale', locale);
    localStorage.setItem('skillshare.tour.completed', 'true');
    localStorage.setItem('skillshare.dashboard.starCta.dismissed', '1');
  }, locale);
  // Block external browser traffic, without fabricating any successful API payload.
  await context.route('**/*', route => new URL(route.request().url()).origin === url ? route.continue() : route.abort('blockedbyclient'));
  await context.tracing.start({ screenshots: true, snapshots: true, sources: true });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('response', async response => {
    if (response.request().method() === 'DELETE' && response.url().includes('/api/repos/')) {
      try {
        manifest.deleteRequests.push({ side, group, url: response.url(), status: response.status(), body: await response.text() });
      } catch (error) { errors.push(`Unable to retain DELETE response: ${error.message}`); }
    }
  });
  const messages = JSON.parse(await readFile(path.join(app, `ui/src/i18n/locales/${locale}.json`), 'utf8'));
  const t = (key, params) => translate(messages, key, params);
  await page.goto(`${url}/?theme=dark`, { waitUntil: 'networkidle' });
  await page.getByRole('heading', { name: t('dashboard.title'), exact: true }).waitFor();
  const cleanStatus = row(page, '_clean').locator('.ss-st');
  await cleanStatus.waitFor();
  assert.equal(await cleanStatus.innerText(), t('dashboard.trackedRepos.clean'));
  await page.evaluate(() => document.fonts.ready);
  await overlays(page);
  // Use one fixed scroll/crop in both sides, retaining navigation and section chrome.
  await page.evaluate(() => window.scrollTo(0, 0));
  async function overview() {
    const response = await context.request.get(`${url}/api/overview`);
    assert.equal(response.status(), 200);
    return response.json();
  }
  async function close() {
    await context.tracing.stop({ path: path.join(output, `${side}-${group}-trace.zip`) });
    await writeFile(path.join(output, `${side}-${group}-backend.log`), fixture.log.join(''));
    assert.deepEqual(errors, [], 'Unhandled browser exceptions');
    await context.close(); activeContext = undefined;
    await fixture.cleanup(); activeFixture = undefined;
  }
  return { page, context, fixture, url, t, overview, close };
}

function row(page, name) { return page.locator('.ss-r').filter({ has: page.getByText(name.replace(/^_/, ''), { exact: true }) }); }
async function openConfirmation(s, name) {
  await click(s.page, row(s.page, name).getByRole('button', { name: s.t('dashboard.trackedRepos.actions'), exact: true }));
  await click(s.page, s.page.getByRole('menuitem', { name: s.t('dashboard.trackedRepos.uninstall'), exact: true }));
  await s.page.getByRole('dialog').waitFor();
}
async function deleteClick(s) {
  const response = s.page.waitForResponse(response => response.request().method() === 'DELETE' && response.url().includes('/api/repos/'));
  await click(s.page, s.page.getByRole('dialog').getByRole('button', { name: s.t('dashboard.trackedRepos.uninstall'), exact: true }));
  const result = await response;
  assert.equal(new URL(result.url()).searchParams.has('force'), false, 'Initial removal must use force=false');
  return result;
}
async function success(s, name, response, movedToTrash) {
  assert.equal(response.status(), 200);
  const result = await response.json();
  assert.equal(result.movedToTrash, movedToTrash);
  assert.equal(result.name, name);
  await s.page.getByRole('dialog').waitFor({ state: 'hidden' });
  const toast = s.t(movedToTrash ? 'dashboard.toast.repoUninstalled' : 'dashboard.toast.repoRecordRemoved', { name: movedToTrash ? name.slice(1) : name });
  await s.page.getByText(toast, { exact: true }).waitFor();
  await row(s.page, name).waitFor({ state: 'hidden' });
  await s.fixture.assertRecordRemoved(name);
  check(`after ${name}: actual response movedToTrash=${movedToTrash}, toast and row invalidation`);
}

async function desktop(side) {
  const s = await session(side, 'desktop', viewport, 'en');
  const { page, fixture } = s;
  const overview = await s.overview();
  await writeFile(path.join(output, `${side}-initial-overview.json`), JSON.stringify(overview, null, 2));
  const declarations = names.filter(name => !['_clean', '_dirty'].includes(name));
  assert.deepEqual(overview.trackedRepos.filter(repo => repo.noCheckout).map(repo => repo.name).sort(), side === 'after' ? declarations.sort() : []);
  if (side === 'before') {
    assert.deepEqual(overview.trackedRepos.map(repo => repo.name).sort(), ['_clean', '_dirty']);
    for (const state of declarationStates) {
      await shot(page, side, state, 'Before: declarations have no dashboard row (same synthetic filesystem)');
    }
    check('before: no impossible declaration rows injected; actual old overview contains only clean/dirty checkouts');
  } else {
    for (const name of declarations) {
      const r = row(page, name);
      assert.ok((await r.innerText()).includes(s.t('dashboard.trackedRepos.noCheckout')));
      assert.equal(await r.getByRole('button', { name: s.t('dashboard.trackedRepos.update'), exact: true }).count(), 0);
      assert.equal(await r.locator('.ss-st').count(), 0);
      assert.equal(/\b\d+ skills?\b/.test(await r.innerText()), false);
    }
    check('after: declaration-only rows have no Update, checkout status or skill count');
    const occupant = await readFile(fixture.occupant, 'utf8');
    const ignore = await readFile(fixture.ignore, 'utf8');
    await shot(page, side, 'absent-row', 'Absent checkout: a removable repository record');
    await shot(page, side, 'occupied-row', 'Occupied path: the record is separate from the folder');
    await caption(page, 'Remove the absent repository record');
    recorder = await startRecording(page);
    for (const name of ['_absent', '_occupied']) {
      await openConfirmation(s, name);
      const text = await page.getByRole('dialog').innerText();
      assert.ok(text.includes(s.t('dashboard.trackedRepos.uninstallConfirm.record', { name })));
      assert.equal(/trash|force/i.test(text), false);
      await shot(page, side, `${name.slice(1)}-confirm`, 'Record-only confirmation: files stay in place');
      const response = await deleteClick(s);
      await success(s, name, response, false);
      await shot(page, side, `${name.slice(1)}-result`, 'Record removed: row gone and actual record-removal toast');
      await page.waitForTimeout(2000);
    }
    assert.equal(await readFile(fixture.occupant, 'utf8'), occupant);
    assert.equal(await readFile(fixture.ignore, 'utf8'), ignore);
    check('after: occupied file and .gitignore remain byte-identical after real record removal');

    await page.route('**/api/repos/_blocked*', route => route.request().method() === 'DELETE'
      ? route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'Synthetic CI record-removal failure' }) }) : route.fallback());
    await openConfirmation(s, '_blocked');
    await shot(page, side, 'blocked-confirm', 'Synthetic error fixture: initial record-only confirmation');
    const blocked = await deleteClick(s);
    assert.equal(blocked.status(), 500);
    await page.getByText('Synthetic CI record-removal failure', { exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: s.t('resources.uninstall.retryForce'), exact: true }).count(), 0);
    assert.ok(await row(page, '_blocked').isVisible());
    assert.ok(JSON.parse(await readFile(fixture.metadata, 'utf8')).entries._blocked);
    await shot(page, side, 'blocked-error', 'Synthetic nonforceable failure: record stays, no force action');
    await page.waitForTimeout(2000);
    await click(page, page.getByRole('dialog').getByRole('button', { name: s.t('common.cancel'), exact: true }));
    check('after: labelled synthetic nonforceable error keeps record/row and offers no force');

    for (const [name, refusal] of [['_restored', null], ['_restored-dirty', 'repo_dirty'], ['_restored-status', 'repo_status_failed']]) {
      await openConfirmation(s, name);
      await shot(page, side, `${name.slice(1)}-confirm`, 'Selected declaration: confirmation still describes the record');
      // Synthetic filesystem restore AFTER selection, with a real local Git checkout.
      await fixture.restore(name, refusal === 'repo_dirty');
      if (refusal === 'repo_status_failed') {
        let first = true;
        await page.route(`**/api/repos/${name}*`, route => {
          if (route.request().method() !== 'DELETE' || !first) return route.fallback();
          first = false;
          return route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: 'Synthetic CI git-status failure on restored checkout', error_code: refusal }) });
        });
      }
      let response = await deleteClick(s);
      if (refusal) {
        assert.equal(response.status(), 409);
        assert.equal((await response.json()).error_code, refusal);
        const dialog = page.getByRole('dialog');
        const warningKey = refusal === 'repo_dirty' ? 'dirty' : 'statusFailed';
        await dialog.getByText(s.t(`dashboard.trackedRepos.uninstallConfirm.${warningKey}`, { name: name.slice(1) }), { exact: true }).waitFor();
        assert.equal((await dialog.innerText()).includes('Nothing on disk is deleted'), false);
        assert.ok(await row(page, name).isVisible());
        assert.ok((await row(page, name).innerText()).includes(s.t('dashboard.trackedRepos.noCheckout')), 'Selected row stays stale until a successful removal refreshes it');
        await shot(page, side, `${name.slice(1)}-refusal`, refusal === 'repo_dirty'
          ? 'Real dirty checkout refusal: explicit force confirmation required'
          : 'Synthetic status failure on real restored checkout: explicit force confirmation');
        const retried = page.waitForResponse(r => r.request().method() === 'DELETE' && r.url().includes(`/api/repos/${name}`));
        await click(page, dialog.getByRole('button', { name: s.t('resources.uninstall.retryForce'), exact: true }));
        response = await retried;
        assert.equal(new URL(response.url()).searchParams.get('force'), 'true');
      }
      await success(s, name, response, true);
      await shot(page, side, `${name.slice(1)}-result`, 'Actual checkout result: moved to trash, checkout toast, row gone');
      await page.waitForTimeout(2000);
    }
  }
  // Existing flows are exercised through the actual controls on BOTH product pins.
  for (const [name, dirty] of [['_clean', false], ['_dirty', true]]) {
    await openConfirmation(s, name);
    await shot(page, side, `${name.slice(1)}-confirm`, `${dirty ? 'Dirty' : 'Clean'} checkout: existing uninstall confirmation`);
    let response = await deleteClick(s);
    if (dirty) {
      assert.equal(response.status(), 409);
      assert.equal((await response.json()).error_code, 'repo_dirty');
      const retry = page.getByRole('dialog').getByRole('button', { name: s.t('resources.uninstall.retryForce'), exact: true });
      await retry.waitFor();
      await shot(page, side, 'dirty-refusal', 'Real dirty checkout: changes require explicit force confirmation');
      const result = page.waitForResponse(r => r.request().method() === 'DELETE' && r.url().includes('/api/repos/_dirty'));
      await click(page, retry);
      response = await result;
      assert.equal(new URL(response.url()).searchParams.get('force'), 'true');
    }
    assert.equal(response.status(), 200);
    assert.equal((await response.json()).movedToTrash, true);
    await page.getByText(s.t('dashboard.toast.repoUninstalled', { name: name.slice(1) }), { exact: true }).waitFor();
    await row(page, name).waitFor({ state: 'hidden' });
    await shot(page, side, `${name.slice(1)}-result`, 'Checkout moved to trash: existing result and toast');
    await page.waitForTimeout(2000);
    check(`${side}: existing ${dirty ? 'dirty explicit-force' : 'clean force=false'} checkout flow`);
  }
  if (recorder) { const finish = recorder; recorder = undefined; await finish(); }
  await s.close();
}

async function localized(side) {
  const long = '_dokumentation-und-automatisierung-fuer-das-entwicklungsteam';
  const s = await session(side, 'narrow-de', narrowViewport, 'de', [...names, long]);
  await shot(s.page, side, 'long-row', 'Deutsch · 1024 CSS px · synthetic long repository name', 'narrow-de');
  if (side === 'after') {
    await openConfirmation(s, long);
    await shot(s.page, side, 'long-confirm', 'Deutsch · record-only confirmation with long repository name', 'narrow-de');
    const layout = await s.page.locator('.ss-dlg').evaluate(dialog => {
      const r = dialog.getBoundingClientRect();
      const body = dialog.querySelector('.db');
      return { x: r.x, right: r.right, bottom: r.bottom, viewport: innerWidth, height: innerHeight, bodyWidth: body.clientWidth, textWidth: body.scrollWidth };
    });
    assert.ok(layout.x >= 0 && layout.right <= layout.viewport && layout.bottom <= layout.height);
    assert.ok(layout.textWidth <= layout.bodyWidth + 1, 'Localized changed message must not overflow horizontally');
    check('after: German long-name record confirmation fits measured 1024px viewport');
  } else {
    assert.equal((await s.overview()).trackedRepos.some(repo => repo.name === long), false);
    await shot(s.page, side, 'long-confirm', 'Deutsch · before has no declaration control (same long-name fixture)', 'narrow-de');
  }
  await s.close();
}

try {
  manifest.ciHead = command('git', ['-C', path.join(workspace, 'harness'), 'rev-parse', 'HEAD']).trim();
  manifest.tools = { node: process.version, pnpm: command('pnpm', ['--version']).trim(), go: command('go', ['version']).trim(), ffmpeg: command('ffmpeg', ['-version']).split('\n')[0] };
  manifest.tools.playwright = JSON.parse(await readFile(new URL('./package.json', pathToFileURL(process.env.CAPTURE_PLAYWRIGHT)), 'utf8')).version;
  manifest.sourceHashes = {};
  for (const file of ['.github/workflows/pr880-hosted-ui.yml', 'scripts/ci-ui/capture.mjs', 'scripts/ci-ui/fixture.mjs', 'scripts/ci-ui/cleanup.mjs']) {
    manifest.sourceHashes[file] = createHash('sha256').update(await readFile(path.join(workspace, 'harness', file))).digest('hex');
  }
  browser = await chromium.launch({ headless: true });
  manifest.tools.chromium = browser.version();
  await desktop('before');
  await desktop('after');
  await localized('before');
  await localized('after');
  const before = manifest.captures.filter(capture => capture.side === 'before');
  const after = manifest.captures.filter(capture => capture.side === 'after');
  assert.equal(before.length, after.length);
  for (const capture of after) {
    const match = before.find(item => item.group === capture.group && item.state === capture.state);
    assert.ok(match, `Missing Before counterpart for ${capture.state}`);
    assert.deepEqual(match.metrics.crop, capture.metrics.crop);
    assert.deepEqual(match.metrics.viewport, capture.metrics.viewport);
    assert.deepEqual(match.metrics.scroll, capture.metrics.scroll);
  }
  manifest.status = 'passed';
} catch (error) {
  manifest.status = 'failed';
  manifest.error = error.stack;
  process.exitCode = 1;
} finally {
  try {
    if (recorder) await recorder();
    if (activeContext) await activeContext.tracing.stop({ path: path.join(output, 'failed-trace.zip') });
  } catch (error) { manifest.mediaCleanupError = error.stack; manifest.status = 'failed'; process.exitCode = 1; }
  try { if (browser) await browser.close(); }
  catch (error) { manifest.browserCleanupError = error.stack; manifest.status = 'failed'; process.exitCode = 1; }
  try {
    if (activeFixture) {
      await writeFile(path.join(output, 'failed-backend.log'), activeFixture.log.join(''));
      await activeFixture.cleanup();
    }
    await rm(stateRoot, { recursive: true, force: true });
    manifest.fixtureCleanup = 'Owned foreground backend stopped; synthetic homes removed';
  } catch (error) { manifest.fixtureCleanup = error.stack; manifest.status = 'failed'; process.exitCode = 1; }
  const checksums = [];
  for (const name of await readdir(output)) {
    if (/\.(png|mp4|zip|log|json)$/.test(name)) checksums.push(`${createHash('sha256').update(await readFile(path.join(output, name))).digest('hex')}  ${name}`);
  }
  // manifest is written last; source identity and partial results survive failures.
  await writeFile(path.join(output, 'manifest.json'), JSON.stringify(manifest, null, 2));
  checksums.push(`${createHash('sha256').update(await readFile(path.join(output, 'manifest.json'))).digest('hex')}  manifest.json`);
  await writeFile(path.join(output, 'SHA256SUMS'), checksums.join('\n') + '\n');
  const pairs = manifest.captures.filter(capture => capture.side === 'after').map(capture => {
    const before = manifest.captures.find(item => item.side === 'before' && item.state === capture.state && item.group === capture.group);
    return `| ${capture.group} / ${capture.state} | ${before?.file ?? 'not captured'} | ${capture.file} |`;
  });
  await writeFile(path.join(output, 'index.md'), `# Hosted PR880 evidence\n\nStatus: ${manifest.status}. Human visual/frame review is still required.\n\n${manifest.provenance}\n\n${manifest.media}\n\nBefore: ${pins.before}\nAfter: ${pins.after}\nCI helper: ${manifest.ciHead}\n\nRecording: removal-sequence.mp4 (H.264).\n\nSynthetic intercepted errors: ${manifest.syntheticInterceptions.join('; ')}.\n\n| State | Before | After |\n| --- | --- | --- |\n${pairs.join('\n')}\n`);
}
