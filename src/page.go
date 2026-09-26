package main

// statusPageHTML is the plugin resource page.
//
// The page renders one compact table row per account and puts the full reading
// into the cell's native tooltip, so everything is visible on hover without
// leaving the single-line layout. Data comes from the unauthenticated
// /status.json resource route by default, which means no management key is
// needed to open the page; a key is only required to trigger a refresh.
const statusPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CommandCode Pool</title>
<style>
:root { color-scheme: light dark; font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif; }
body { margin: 0; padding: 20px; background: Canvas; color: CanvasText; font-size: 14px; }
h1 { font-size: 19px; margin: 0 0 4px; }
h2 { font-size: 15px; margin: 0 0 10px; }
.card { border: 1px solid color-mix(in srgb, CanvasText 18%, transparent); border-radius: 10px; padding: 12px 14px; margin-bottom: 14px; }
.row { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
input[type=password] { padding: 5px 9px; border-radius: 6px; border: 1px solid color-mix(in srgb, CanvasText 25%, transparent); min-width: 220px; background: transparent; color: inherit; }
button { padding: 5px 12px; border-radius: 6px; border: 1px solid color-mix(in srgb, CanvasText 25%, transparent); background: transparent; color: inherit; cursor: pointer; }
button:hover { background: color-mix(in srgb, CanvasText 8%, transparent); }
table.grid { border-collapse: collapse; width: 100%; font-size: 13px; }
table.grid th, table.grid td { text-align: left; padding: 7px 10px; border-bottom: 1px solid color-mix(in srgb, CanvasText 12%, transparent); vertical-align: middle; white-space: nowrap; }
table.grid th { font-weight: 600; opacity: .75; font-size: 12px; }
table.grid tr:hover td { background: color-mix(in srgb, CanvasText 5%, transparent); }
.bar { display: inline-block; width: 58px; height: 7px; border-radius: 4px; background: color-mix(in srgb, CanvasText 12%, transparent); overflow: hidden; vertical-align: middle; margin-right: 6px; }
.bar > i { display: block; height: 100%; background: #16a34a; }
.bar > i.warn { background: #d97706; } .bar > i.bad { background: #dc2626; }
.ok { color: #16a34a; } .bad { color: #dc2626; } .warn { color: #d97706; }
.muted { opacity: .65; font-size: 12px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.badge { display: inline-block; padding: 0 6px; border-radius: 999px; font-size: 11px; border: 1px solid currentColor; }
#error { color: #dc2626; margin: 8px 0; min-height: 16px; font-size: 13px; }
.tip { cursor: help; }
</style>
</head>
<body>
<h1>CommandCode Pool</h1>
<div class="muted" id="meta"></div>
<div class="card">
  <div class="row">
    <label class="muted">Management key (optional — only needed to refresh)
      <input type="password" id="key" placeholder="leave empty to view read-only" autocomplete="off">
    </label>
    <button id="load">Reload</button>
    <button id="refresh" hidden>Refresh all</button>
    <button id="togglePlans">Plan catalog</button>
    <span class="muted" id="mode"></span>
  </div>
  <div id="error"></div>
</div>
<div id="plans" hidden></div>
<div id="content" class="muted">Loading…</div>
<script>
var RES = '/v0/resource/plugins/commandcode-pool';
var MGMT = '/v0/management/plugins/commandcode-pool';
var keyInput = document.getElementById('key');
var state = { warn: 80, critical: 95 };

function authHeaders() {
  var value = keyInput.value.trim();
  var h = { 'Content-Type': 'application/json' };
  if (value) h['Autho' + 'rization'] = 'Bearer ' + value;
  return h;
}
function deobfuscatePayload(payload) {
  var prefix = 'enc::v1::';
  if (!payload || payload.indexOf(prefix) !== 0) return payload;
  var binary = atob(payload.slice(prefix.length));
  var key = 'cli-proxy-api-webui::secure-storage|' + location.host + '|' + navigator.userAgent;
  var keyBytes = new TextEncoder().encode(key);
  var bytes = new Uint8Array(binary.length);
  for (var i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i) ^ keyBytes[i % keyBytes.length];
  return new TextDecoder().decode(bytes);
}
function readStoredKey() {
  try {
    var raw = localStorage.getItem('cli-proxy-auth');
    if (raw) {
      var payload = JSON.parse(deobfuscatePayload(raw));
      var st = (payload && typeof payload.state === 'object' && payload.state !== null) ? payload.state : payload;
      if (st && typeof st.managementKey === 'string' && st.managementKey) return st.managementKey;
    }
  } catch (e) {}
  try {
    var legacy = localStorage.getItem('managementKey');
    if (legacy) return legacy;
  } catch (e) {}
  return null;
}
function esc(s) {
  return String(s === undefined || s === null ? '' : s).replace(/[&<>"]/g, function (c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c];
  });
}
function fmtTime(iso) { return iso ? new Date(iso).toLocaleString() : ''; }
function fmtNum(v, digits) {
  if (v === undefined || v === null) return '-';
  var d = (digits === undefined) ? 2 : digits;
  return Number(v).toLocaleString(undefined, { maximumFractionDigits: d });
}
function fmtBool(v) {
  if (v === undefined || v === null) return 'null (no signal)';
  return v ? 'true' : 'false';
}
function pctClass(p) {
  if (p === undefined || p === null) return '';
  if (p >= state.critical) return 'bad';
  if (p >= state.warn) return 'warn';
  return '';
}
function bar(percent) {
  var p = (percent === undefined || percent === null) ? 0 : Math.max(0, Math.min(100, percent));
  return '<span class="bar"><i class="' + pctClass(percent) + '" style="width:' + p + '%"></i></span>';
}
function cell(inner, tip, cls) {
  return '<td' + (cls ? ' class="' + cls + '"' : '') + (tip ? ' title="' + esc(tip) + '"' : '') + '>' + inner + '</td>';
}
function join(lines) { return lines.filter(function (l) { return l !== '' && l !== undefined && l !== null; }).join('\n'); }

function windowCell(w) {
  if (!w || w.used === undefined) {
    return { text: '<span class="muted">n/a</span>', tip: 'no /alpha/billing/credits reading for this window yet' };
  }
  var pct = w.used_percent;
  var text = bar(pct) + '<span class="mono ' + pctClass(pct) + '">' + fmtNum(pct, 1) + '%</span>';
  var tip = join([
    'used:  $' + fmtNum(w.used, 6),
    'cap:   $' + fmtNum(w.cap, 2),
    'headroom: $' + fmtNum(w.headroom, 6),
    'percent: ' + fmtNum(pct, 2) + '%',
    'exceeded: ' + fmtBool(w.exceeded),
    'blocked: ' + (w.blocked ? 'yes' : 'no') + (w.blocked ? '  (effective: ' + (w.blocked_effective ? 'yes' : 'no — prepaid credits bypass caps') + ')' : ''),
    'reset at: ' + (w.reset_at ? fmtTime(w.reset_at) : 'unknown'),
    'resets in: ' + (w.resets_in || 'unknown'),
  ]);
  return { text: text, tip: tip };
}

function statusBadge(status) {
  if (!status) return '';
  return ' <span class="badge ' + (status === 'active' ? 'ok' : 'bad') + '">' + esc(status) + '</span>';
}

function accountRow(a) {
  var plan = a.plan || {};
  var bal = a.balance || {};
  var id = a.identity || {};

  var badges = '';
  if (a.disabled) badges += ' <span class="badge muted">disabled</span>';
  if (a.stale) badges += ' <span class="badge warn">stale</span>';
  if (plan.shared_keys > 1) badges += ' <span class="badge warn">shared pool</span>';

  var acctText = '<b>' + esc(a.name) + '</b>' + badges
    + (a.key_suffix ? '<br><span class="muted">…' + esc(a.key_suffix) + '</span>' : '');
  var acctTip = join([
    'providers: ' + ((a.providers && a.providers.length) ? a.providers.join(', ') : '-'),
    'user: ' + ([id.user_name, id.name].filter(Boolean).join(' / ') || '-'),
    id.email ? 'email: ' + id.email : '',
    id.org_id ? 'org: ' + id.org_id : '',
    'disabled: ' + (a.disabled ? 'yes' : 'no'),
    'stale: ' + (a.stale ? 'yes' : 'no'),
    'refreshed: ' + (a.refreshed_at ? fmtTime(a.refreshed_at) : 'never'),
    (a.data_age_seconds !== undefined && a.data_age_seconds !== null) ? 'data age: ' + a.data_age_seconds + 's' : '',
    a.attempted_at ? 'last attempt: ' + fmtTime(a.attempted_at) : '',
  ]);

  var planText = plan.id
    ? '<b>' + esc(plan.name || plan.id) + '</b>' + statusBadge(plan.status)
    : '<span class="muted">no subscription</span>';
  var planTip = plan.id ? join([
    'planId: ' + plan.id,
    plan.marketing_name ? 'marketing name: ' + plan.marketing_name : '',
    plan.known === false ? 'catalog: UNKNOWN planId — raw API values only' : 'catalog: known plan',
    'status: ' + (plan.status || '-'),
    plan.quantity !== undefined ? 'quantity: ' + plan.quantity : '',
    'cancel at period end: ' + (plan.cancel_at_period_end ? 'yes' : 'no'),
    plan.subscription_id ? 'subscription: ' + plan.subscription_id : '',
    plan.price_id ? 'price: ' + plan.price_id : '',
    plan.created_at ? 'created: ' + fmtTime(plan.created_at) : '',
    plan.pending_phase ? 'pending phase: ' + JSON.stringify(plan.pending_phase) : '',
  ]) : 'no active subscription reported by /alpha/billing/subscriptions';

  var credText, credTip;
  if (bal.monthly_remaining === undefined) {
    credText = '<span class="muted">n/a</span>';
    credTip = 'no /alpha/billing/credits reading yet';
  } else {
    var inc = bal.monthly_included;
    var cons = bal.monthly_consumed_percent;
    credText = '<span class="mono">$' + fmtNum(bal.monthly_remaining, 2)
      + (inc === undefined ? '' : ' / $' + fmtNum(inc, 0)) + '</span>'
      + (inc === undefined ? '' : '<br>' + bar(cons) + '<span class="mono ' + pctClass(cons) + '">' + fmtNum(cons, 1) + '%</span>');
    credTip = join([
      'monthly remaining: $' + fmtNum(bal.monthly_remaining, 6),
      inc === undefined ? 'included allowance: unknown (planId not in catalog)' : 'included allowance: $' + fmtNum(inc, 2),
      bal.monthly_consumed !== undefined ? 'consumed: $' + fmtNum(bal.monthly_consumed, 6) + '  (' + fmtNum(cons, 2) + '%)' : '',
      'purchased credits: $' + fmtNum(bal.purchased_credits, 6) + '  (exempt from window caps)',
      'free credits: $' + fmtNum(bal.free_credits, 6),
      'total remaining: $' + fmtNum(bal.total_remaining, 6),
      'low-credit warning: ' + (bal.below_threshold ? 'ACTIVE (threshold $' + fmtNum(bal.credit_threshold, 2) + ')' : 'off'),
      'rolling windows enforced: ' + (bal.limited ? 'yes' : 'no'),
      'window exceeded flag: ' + fmtBool(bal.window_exceeded),
      bal.sandbox_access ? 'sandbox access: yes' + (bal.sandbox_minutes !== undefined ? ' (' + fmtNum(bal.sandbox_minutes, 0) + ' min)' : '') : '',
    ]);
  }

  var perText, perTip;
  if (!plan.current_period_start) {
    perText = '<span class="muted">n/a</span>';
    perTip = 'no billing period reported';
  } else {
    var dr = plan.days_remaining;
    perText = '<span class="mono">' + (dr === undefined || dr === null ? '-' : dr + 'd') + '</span>'
      + '<br>' + bar(plan.period_elapsed_percent) + '<span class="muted">' + fmtNum(plan.period_elapsed_percent, 0) + '%</span>';
    perTip = join([
      'billing period start: ' + fmtTime(plan.current_period_start),
      'billing period end:   ' + fmtTime(plan.current_period_end),
      'days remaining: ' + (dr === undefined || dr === null ? '-' : dr),
      'period elapsed: ' + fmtNum(plan.period_elapsed_percent, 2) + '%',
      'monthly credits reset at the period end',
    ]);
  }

  var s = a.period_summary;
  var reqText, reqTip;
  if (!s) {
    reqText = '<span class="muted">n/a</span>';
    reqTip = 'usage summary unavailable or disabled (include-usage-summary)';
  } else {
    reqText = '<span class="mono">' + fmtNum(s.requests, 0) + '</span>'
      + ' <span class="muted">·</span> <span class="' + (s.success_rate >= 99 ? 'ok' : 'warn') + '">' + fmtNum(s.success_rate, 0) + '%</span>';
    reqTip = join([
      'requests: ' + s.requests + '  (completed ' + s.completed + ', failed ' + s.failed + ')',
      'success rate: ' + fmtNum(s.success_rate, 2) + '%',
      'tokens in / out / total: ' + fmtNum(s.tokens_in, 0) + ' / ' + fmtNum(s.tokens_out, 0) + ' / ' + fmtNum(s.tokens_total, 0),
      'cost: $' + fmtNum(s.total_cost, 6) + '   average: $' + fmtNum(s.average_cost, 6),
      'credits: $' + fmtNum(s.total_credits, 6) + '  (monthly $' + fmtNum(s.total_monthly_credits, 6)
        + ', purchased $' + fmtNum(s.total_purchased_credits, 6) + ', free $' + fmtNum(s.total_free_credits, 6) + ')',
      'requested since: ' + (s.since || '-'),
      'effective since: ' + (s.since_effective || '-') + '   [' + (s.granularity || 'unknown') + ' bucketing]',
      'period basis: ' + (s.period_basis || '-'),
    ]);
  }

  var errs = a.errors ? Object.keys(a.errors) : [];
  var healthText = errs.length
    ? '<span class="bad">' + errs.length + ' err</span>'
    : '<span class="ok">ok</span>';
  var healthTip = errs.length
    ? join(errs.map(function (k) { return k + ': ' + a.errors[k]; }))
    : 'all /alpha/* endpoints healthy';
  healthTip += '\nrefreshed: ' + (a.refreshed_at ? fmtTime(a.refreshed_at) : 'never');
  if (a.data_age_seconds !== undefined && a.data_age_seconds !== null) healthTip += '\ndata age: ' + a.data_age_seconds + 's';
  if (a.stale) healthTip += '\nSTALE: older than usage-stale-after';

  var w5 = windowCell((a.windows || {})['5h']);
  var ww = windowCell((a.windows || {}).weekly);

  return '<tr>'
    + cell(acctText, acctTip)
    + cell(planText, planTip)
    + cell(w5.text, '5-hour window\n' + w5.tip, 'tip')
    + cell(ww.text, 'weekly window\n' + ww.tip, 'tip')
    + cell(credText, credTip, 'tip')
    + cell(perText, perTip, 'tip')
    + cell(reqText, reqTip, 'tip')
    + cell(healthText, healthTip, 'tip')
    + '</tr>';
}

function render(status, mode) {
  state.warn = status.warn_percent || 80;
  state.critical = status.critical_percent || 95;
  document.getElementById('meta').textContent =
    'v' + status.version + ' · ' + status.api_base_url
    + ' · refresh ' + status.refresh_interval
    + ' · stale after ' + status.stale_after
    + ' · ' + fmtTime(status.generated_at);
  document.getElementById('mode').textContent = mode === 'management'
    ? 'management view'
    : 'read-only view · no management key required · hover any cell for details';

  var html = '';
  if (status.config_error) html += '<div class="card bad">config error: ' + esc(status.config_error) + '</div>';
  var shared = status.shared_subscriptions || [];
  for (var i = 0; i < shared.length; i++) {
    var g = shared[i];
    html += '<div class="card warn">Shared credit pool: ' + esc(g.accounts.join(', '))
      + ' share one subscription (' + esc(g.plan_id || 'unknown plan') + ') — one balance for all of them.</div>';
  }
  if (!status.accounts.length) {
    html += '<div class="card muted">No CommandCode API keys discovered. Check cpa-config-path and that a credential base URL contains commandcode.ai.</div>';
    document.getElementById('content').innerHTML = html;
    return;
  }
  html += '<div class="card"><table class="grid"><tr>'
    + '<th>Account</th><th>Plan</th><th>5h</th><th>Weekly</th><th>Monthly credits</th><th>Period</th><th>Requests</th><th>Health</th>'
    + '</tr>';
  for (var j = 0; j < status.accounts.length; j++) html += accountRow(status.accounts[j]);
  html += '</table><div class="muted" style="margin-top:8px">Hover any cell for the full reading. Percentages are credit/USD-equivalent value against the plan window cap.</div></div>';
  document.getElementById('content').innerHTML = html;
}

async function fetchJSON(url, options) {
  var resp = await fetch(url, options || {});
  if (!resp.ok) throw new Error('HTTP ' + resp.status + (resp.status === 401 ? ' (bad management key?)' : ''));
  return resp.json();
}

async function load() {
  var err = document.getElementById('error');
  err.textContent = '';
  var key = keyInput.value.trim();
  try { sessionStorage.setItem('ccp-key', key); } catch (e) {}
  document.getElementById('refresh').hidden = !key;

  var status = null;
  var mode = 'public';
  if (key) {
    try {
      status = await fetchJSON(MGMT + '/status', { headers: authHeaders() });
      mode = 'management';
    } catch (e) {
      err.textContent = 'management key not accepted (' + e.message + '); showing the read-only view';
    }
  }
  if (!status) {
    try {
      status = await fetchJSON(RES + '/status.json');
    } catch (e) {
      throw new Error('no data available: the read-only endpoint failed (' + e.message
        + '). Either enable public-status or enter the management key.');
    }
  }
  render(status, mode);
}

async function plansHTML() {
  var data;
  try { data = await fetchJSON(RES + '/plans'); }
  catch (e) { data = await fetchJSON(MGMT + '/plans', { headers: authHeaders() }); }
  var html = '<div class="card"><h2>Plan catalog</h2><table class="grid"><tr><th>planId</th><th>Name</th><th>Marketing</th><th>Monthly credits</th><th>5h cap</th><th>Weekly cap</th></tr>';
  for (var i = 0; i < data.plans.length; i++) {
    var p = data.plans[i];
    html += '<tr><td class="mono">' + esc(p.id) + '</td><td>' + esc(p.name) + '</td><td>' + esc(p.marketing_name || '')
      + '</td><td>' + (p.includes_credits ? '$' + fmtNum(p.monthly_credits, 2) : 'pay-as-you-go')
      + '</td><td>' + (p.five_hour_cap === undefined ? '-' : '$' + fmtNum(p.five_hour_cap, 2))
      + '</td><td>' + (p.weekly_cap === undefined ? '-' : '$' + fmtNum(p.weekly_cap, 2)) + '</td></tr>';
  }
  return html + '</table><div class="muted" style="margin-top:8px">Reference data from the CommandCode CLI bundle and pricing page; the live API remains authoritative for caps and balances.</div></div>';
}

document.getElementById('load').addEventListener('click', function () {
  load().catch(function (e) { document.getElementById('error').textContent = String(e); });
});
document.getElementById('refresh').addEventListener('click', async function () {
  try {
    await fetchJSON(MGMT + '/refresh', { method: 'POST', headers: authHeaders(), body: '{}' });
    setTimeout(function () { load().catch(function () {}); }, 1500);
  } catch (e) { document.getElementById('error').textContent = String(e); }
});
document.getElementById('togglePlans').addEventListener('click', async function () {
  var el = document.getElementById('plans');
  if (el.hidden) {
    try { el.innerHTML = await plansHTML(); el.hidden = false; }
    catch (e) { document.getElementById('error').textContent = String(e); }
  } else {
    el.hidden = true;
  }
});
keyInput.addEventListener('keydown', function (e) {
  if (e.key === 'Enter') load().catch(function (err) { document.getElementById('error').textContent = String(err); });
});

try { keyInput.value = sessionStorage.getItem('ccp-key') || ''; } catch (e) {}
if (!keyInput.value) {
  var stored = readStoredKey();
  if (stored) keyInput.value = stored;
}
document.getElementById('refresh').hidden = !keyInput.value.trim();
load().catch(function (e) { document.getElementById('error').textContent = String(e); });
</script>
</body>
</html>`
