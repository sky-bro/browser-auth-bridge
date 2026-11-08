// This adapter performs only canonical same-origin GETs. It never executes the
// product page's control scripts or follows renew/reboot/SSO/form actions.
const serviceID = args['service-id'] || '';
if (serviceID && !/^[1-9]\d*$/.test(serviceID)) return {error: {code: 'invalid_args'}};
const origin = 'https://www.akkocloud.com';
const fetchedAt = new Date().toISOString();
const normalize = text => String(text || '').replace(/\s+/g, ' ').trim();
const loginPage = doc => !!doc.querySelector('form[action*="dologin.php"],form#frmLogin');
const error = code => ({error: {code}});

function amount(text) {
  const raw = normalize(text);
  const m = raw.match(/^(\d+(?:,\d{3})*(?:\.\d+)?)\s*(B|[kKmMgGtTpPeE][iI]?B)$/);
  if (!m) return {bytes: null, raw: raw || null, unit: null};
  const unit = m[2].toUpperCase();
  const index = 'BKMGTPE'.indexOf(unit[0]);
  const base = unit.includes('I') ? 1024 : 1000;
  const bytes = Math.round(Number(m[1].replace(/,/g, '')) * Math.pow(base, index));
  return {bytes: Number.isSafeInteger(bytes) ? bytes : null, raw, unit: m[2]};
}

function valueBefore(text, label) {
  // Do not turn an unsupported decimal comma or negative amount into a smaller
  // positive suffix (for example, "1,5 GB" must not become "5 GB").
  const match = text.match(new RegExp('(?:^|[^\\d.,+\\-])(\\d+(?:,\\d{3})*(?:\\.\\d+)?\\s*(?:[KMGTPE]i?B|B))\\s*' + label, 'i'));
  return amount(match ? match[1] : '');
}

function explicitDate(doc, pattern) {
  // Billing/renewal dates are intentionally excluded. A date without a timezone
  // stays raw; do not invent a midnight/timezone for the provider.
  for (const row of doc.querySelectorAll('tr')) {
    const cells = [...row.querySelectorAll('td,th')];
    if (cells.length < 2 || !pattern.test(normalize(cells[0].textContent))) continue;
    const raw = normalize(cells[1].textContent);
    const m = raw.match(/\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:\d{2})\b/);
    const day = m && new Date(m[0].slice(0, 10) + 'T00:00:00Z');
    if (m && Number.isFinite(Date.parse(m[0])) && Number.isFinite(day.getTime()) &&
        day.toISOString().slice(0, 10) === m[0].slice(0, 10) && Number(m[0].slice(11, 13)) < 24) {
      return {raw, at: new Date(m[0]).toISOString()};
    }
    return {raw: raw || null, at: null};
  }
  return {raw: null, at: null};
}

function traffic(doc) {
  let used = amount(''), limit = amount(''), remaining = amount('');
  for (const row of doc.querySelectorAll('tr')) {
    const cells = [...row.querySelectorAll('td,th')];
    if (cells.length < 2 || !/^(?:流量统计|流量使用|Bandwidth(?:\s+Usage)?|Traffic(?:\s+Usage)?)$/i.test(normalize(cells[0].textContent))) continue;
    const value = normalize(cells[1].textContent);
    used = valueBefore(value, '(?:已用|已使用|used)');
    limit = valueBefore(value, '(?:总量|总流量|total|allowance)');
    // The observed module puts the allocation in a strong element; do not use
    // percentages or unrelated resource rows when a label is unavailable.
    if (limit.bytes === null) {
      const strong = [...cells[1].querySelectorAll('strong,b')].map(e => amount(e.textContent)).filter(v => v.bytes !== null);
      if (strong.length === 1) limit = strong[0];
    }
    remaining = valueBefore(value, '(?:可用|剩余|available|remaining|free)');
    break;
  }
  const reset = explicitDate(doc, /^(?:下次流量重置(?:时间|日期)?|流量重置(?:时间|日期)|Next\s+(?:Traffic|Bandwidth)\s+Reset)$/i);
  const start = explicitDate(doc, /^(?:流量周期开始(?:时间|日期)?|(?:Traffic|Bandwidth)\s+Period\s+Start)$/i);
  return {
    used_bytes: used.bytes, limit_bytes: limit.bytes, remaining_bytes: remaining.bytes,
    upload_bytes: null, download_bytes: null,
    reset_at: reset.at, period_start: start.at, direction: 'both',
    raw_unit: used.unit === limit.unit ? used.unit : null,
    raw_used: used.raw, raw_limit: limit.raw, raw_remaining: remaining.raw,
    raw_reset: reset.raw, raw_period_start: start.raw,
    precision: 'display-rounded', unit_basis: 'label-SI-or-IEC'
  };
}

async function page(path) {
  const response = await ctx.fetch(path);
  if (!response.ok) return {error: 'http_error'};
  const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
  return loginPage(doc) ? {error: 'login_required'} : {doc};
}

const list = await page('/clientarea.php?action=services');
if (list.error) return error(list.error);
if (!list.doc.querySelector('a[href*="logout.php"],a[href*="action=productdetails"]')) return error('unexpected_page');
const services = new Map();
for (const row of list.doc.querySelectorAll('tr')) {
  const links = [...row.querySelectorAll('a[href*="action=productdetails"]')];
  let id = null;
  for (const link of links) {
    let url;
    try { url = new URL(link.getAttribute('href'), origin); } catch (_) { continue; }
    const candidate = url.searchParams.get('id') || '';
    if (url.origin === origin && url.pathname === '/clientarea.php' && url.searchParams.get('action') === 'productdetails' && /^[1-9]\d*$/.test(candidate)) {
      id = candidate;
      break;
    }
  }
  if (!id || (serviceID && id !== serviceID)) continue;
  const cells = [...row.querySelectorAll('td')];
  const title = cells[1]?.querySelector('strong,b');
  const status = row.querySelector('.status,.label-status,.label');
  services.set(id, {
    id, name: normalize(title?.textContent || cells[1]?.textContent) || null,
    status: normalize(status?.textContent || cells[4]?.textContent) || null
  });
}
if (serviceID && !services.has(serviceID)) return error('service_not_found');
if (services.size > 20) return error('too_many_services');
for (const service of services.values()) {
  const detail = await page('/clientarea.php?action=productdetails&id=' + service.id);
  if (detail.error) return error(detail.error);
  service.traffic = traffic(detail.doc);
  service.warnings = service.traffic.used_bytes === null || service.traffic.limit_bytes === null ? ['traffic_unavailable'] : [];
}
return {schema_version: 1, provider: 'akkocloud', fetched_at: fetchedAt, services: [...services.values()]};
