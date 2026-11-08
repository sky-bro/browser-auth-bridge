const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function cell(text, strong = []) {
  return {textContent: text, querySelector: () => null,
    querySelectorAll: selector => selector === 'strong,b' ? strong.map(textContent => ({textContent})) : []};
}
function row(label, value, strong = []) {
  const cells = [cell(label), cell(value, strong)];
  return {querySelectorAll: selector => selector === 'td,th' ? cells : [], querySelector: () => null};
}
function serviceRow(id, origin = '', name = 'Fixture VPS', status = 'Active') {
  const link = {getAttribute: () => origin + '/clientarea.php?action=productdetails&id=' + id};
  const cells = [cell(''), cell(name), cell('$10/year'), cell('2099-04-20'), cell(status)];
  return {querySelectorAll: selector => selector.startsWith('a[') ? [link] : selector === 'td' ? cells : [],
    querySelector: () => ({textContent: status})};
}
function document(rows = [], {login = false, auth = true} = {}) {
  return {querySelectorAll: selector => selector === 'tr' ? rows : [], querySelector: selector => {
    if (selector.startsWith('form')) return login ? {} : null;
    return auth ? {} : null;
  }};
}
async function run(args = {}, options = {}, command = 'traffic') {
  const requests = [];
  const docs = [options.list || document([serviceRow('123')]), options.detail || document([
    row('流量统计', '500.29 GB 已用 500 GB 总量 / 0 B 可用', ['500 GB']),
    row('下次付款日期', '2099-04-20'), row('root password', 'fixture-password-never-export'),
  ])];
  const scope = {args, URL, Date, Map, Number, DOMParser: class {
    parseFromString(text) { return docs[Number(text)]; }
  }, ctx: {fetch: async url => {
    requests.push(url);
    return {ok: options.ok !== false, text: async () => String(requests.length - 1)};
  }}};
  const script = fs.readFileSync(path.join(__dirname, 'builtin/akkocloud', command + '.js'), 'utf8');
  const value = await vm.runInNewContext('(async()=>{\n' + script + '\n})()', scope);
  return {value: JSON.parse(JSON.stringify(value)), requests};
}

test('actual module layout projects only service traffic and preserves displayed units', async () => {
  const {value, requests} = await run();
  assert.deepEqual(requests, ['/clientarea.php?action=services', '/clientarea.php?action=productdetails&id=123']);
  assert.equal(value.schema_version, 1);
  assert.equal(value.provider, 'akkocloud');
  assert.ok(Number.isFinite(Date.parse(value.fetched_at)));
  assert.equal(value.services[0].id, '123');
  assert.equal(value.services[0].traffic.used_bytes, 500290000000);
  assert.equal(value.services[0].traffic.limit_bytes, 500000000000);
  assert.equal(value.services[0].traffic.remaining_bytes, 0);
  assert.equal(value.services[0].traffic.raw_used, '500.29 GB');
  assert.equal(value.services[0].traffic.raw_unit, 'GB');
  assert.equal(value.services[0].traffic.precision, 'display-rounded');
  assert.equal(value.services[0].traffic.upload_bytes, null);
  assert.equal(value.services[0].traffic.download_bytes, null);
  assert.equal(value.services[0].traffic.reset_at, null);
  assert.equal(value.services[0].traffic.period_start, null);
  assert.ok(!JSON.stringify(value).includes('fixture-password'));
  assert.ok(!JSON.stringify(value).includes('2099-04-20'));
});
test('IEC units differ from SI and ambiguous bare G is never converted', async () => {
  const iec = await run({}, {detail: document([row('Bandwidth Usage', '1.5 GiB used 500 GiB total / 498.5 GiB remaining')])});
  assert.equal(iec.value.services[0].traffic.used_bytes, 1610612736);
  assert.equal(iec.value.services[0].traffic.limit_bytes, 536870912000);
  const ambiguous = await run({}, {detail: document([row('流量统计', '1 G 已用 500 G 总量')])});
  assert.equal(ambiguous.value.services[0].traffic.used_bytes, null);
  assert.equal(ambiguous.value.services[0].traffic.limit_bytes, null);
  assert.deepEqual(ambiguous.value.services[0].warnings, ['traffic_unavailable']);
});
test('unsupported numeric formats and bit units stay unknown instead of matching a suffix', async () => {
  for (const value of ['1,5 GB', '-5 GB', '+5 GB', '500 Gb']) {
    const result = await run({}, {detail: document([row('流量统计', value + ' 已用 500 GB 总量')])});
    assert.equal(result.value.services[0].traffic.used_bytes, null, value);
    assert.deepEqual(result.value.services[0].warnings, ['traffic_unavailable']);
  }
  const grouped = await run({}, {detail: document([row('流量统计', '1,234.5 MB 已用 500 GB 总量')])});
  assert.equal(grouped.value.services[0].traffic.used_bytes, 1234500000);
});
test('invalid ids fail before any browser request and ids must belong to list', async () => {
  for (const id of ['../private', '123&modop=custom&a=reboot', '0', '-1']) {
    const r = await run({'service-id': id});
    assert.equal(r.value.error.code, 'invalid_args');
    assert.equal(r.requests.length, 0);
  }
  const unknown = await run({'service-id': '999'});
  assert.equal(unknown.value.error.code, 'service_not_found');
  assert.deepEqual(unknown.requests, ['/clientarea.php?action=services']);
});
test('cross-origin and lookalike service links never become detail requests', async () => {
  const result = await run({}, {list: document([serviceRow('789', 'https://other.example.test')])});
  assert.deepEqual(result.value.services, []);
  assert.deepEqual(result.requests, ['/clientarea.php?action=services']);
});
test('login expiry and unexpected pages do not return account page contents', async () => {
  const login = await run({}, {list: document([], {login: true, auth: false})});
  assert.deepEqual(login.value, {error: {code: 'login_required'}});
  const unknown = await run({}, {list: document([], {auth: false})});
  assert.deepEqual(unknown.value, {error: {code: 'unexpected_page'}});
  const expired = await run({}, {detail: document([], {login: true, auth: false})});
  assert.deepEqual(expired.value, {error: {code: 'login_required'}});
});
test('missing traffic stays null, rather than becoming zero or another resource usage', async () => {
  const result = await run({}, {detail: document([row('Disk Usage', '20 GB 已用 40 GB 总量')])});
  assert.equal(result.value.services[0].traffic.used_bytes, null);
  assert.equal(result.value.services[0].traffic.limit_bytes, null);
  assert.equal(result.value.services[0].traffic.remaining_bytes, null);
});
test('only explicit traffic dates with timezone become normalized timestamps', async () => {
  const detail = document([row('流量统计', '100 MB used 500 GB total'),
    row('Next Traffic Reset', '2026-11-17T00:00:00+08:00'),
    row('Traffic Period Start', '2026-10-17T00:00:00+08:00')]);
  const result = await run({}, {detail});
  assert.equal(result.value.services[0].traffic.reset_at, '2026-11-16T16:00:00.000Z');
  assert.equal(result.value.services[0].traffic.period_start, '2026-10-16T16:00:00.000Z');
  const nozone = await run({}, {detail: document([row('Next Traffic Reset', '2026-11-17')])});
  assert.equal(nozone.value.services[0].traffic.reset_at, null);
  assert.equal(nozone.value.services[0].traffic.raw_reset, '2026-11-17');
});
test('invalid provider calendar dates are not normalized into a different reset date', async () => {
  for (const date of ['2026-02-30T00:00:00+08:00', '2026-11-17T24:00:00+08:00']) {
    const result = await run({}, {detail: document([row('Next Traffic Reset', date)])});
    assert.equal(result.value.services[0].traffic.reset_at, null);
    assert.equal(result.value.services[0].traffic.raw_reset, date);
  }
});
test('service selection does not fetch or return unrelated details', async () => {
  const result = await run({'service-id': '123'}, {list: document([serviceRow('123'), serviceRow('456')])});
  assert.equal(result.value.services.length, 1);
  assert.equal(result.requests.length, 2);
});
test('HTTP failures return a bounded error without raw response text', async () => {
  const result = await run({}, {ok: false});
  assert.deepEqual(result.value, {error: {code: 'http_error'}});
});
