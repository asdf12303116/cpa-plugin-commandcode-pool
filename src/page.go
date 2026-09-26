package main

// statusPageHTML is the plugin resource page.
//
// The page renders one compact table row per account and puts the full reading
// into each cell's native tooltip. It loads by itself on open: the management
// key is taken from CPA Manager Plus' persisted auth store, from this page's own
// remembered value, or from the current tab, and a key typed or pasted into the
// field triggers the load automatically, so the Load button is only a fallback.
//
// CommandCode throttles on credit/USD-equivalent value rather than request
// quotas, so every column is amount-based. The columns also depend on the plan
// kind: subscription plans are limited by the rolling 5h/weekly windows, while
// pay-as-you-go (Provider) accounts have no windows and are limited by their
// credit balance. Request counts only appear as secondary diagnostics.
const statusPageHTML = `<!doctype html>
<html lang="en">
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
    <label class="muted">Management key
      <input type="password" id="key" placeholder="loaded automatically when available" autocomplete="off">
    </label>
    <button id="load">Reload</button>
    <button id="refresh" hidden>Refresh all</button>
    <button id="togglePlans">Plan catalog</button>
    <button id="forget" hidden title="Remove the remembered key from this browser">Forget key</button>
    <span class="muted" id="keySource"></span>
  </div>
  <div id="error"></div>
</div>
<div id="plans" hidden></div>
<div id="content" class="muted">Loading…</div>
<script>
var MGMT = '/v0/management/plugins/commandcode-pool';
var STORE_KEY = 'ccp-key';
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

// readStoredKey resolves a management key without any user interaction:
// CPA Manager Plus' persisted auth store first, then the key this page
// remembered, then the current tab.
function readStoredKey() {
  try {
    var raw = localStorage.getItem('cli-proxy-auth');
    if (raw) {
      var payload = JSON.parse(deobfuscatePayload(raw));
      var st = (payload && typeof payload.state === 'object' && payload.state !== null) ? payload.state : payload;
      if (st && typeof st.managementKey === 'string' && st.managementKey) {
        return { key: st.managementKey, source: 'CPA Manager Plus' };
      }
    }
  } catch (e) {}
  try {
    var legacy = localStorage.getItem('managementKey');
    if (legacy) return { key: legacy, source: 'CPA Manager Plus' };
  } catch (e) {}
  try {
    var own = localStorage.getItem(STORE_KEY);
    if (own) return { key: own, source: 'this browser' };
  } catch (e) {}
  try {
    var tab = sessionStorage.getItem(STORE_KEY);
    if (tab) return { key: tab, source: 'this tab' };
  } catch (e) {}
  return null;
}
function rememberKey(key) {
  try { localStorage.setItem(STORE_KEY, key); } catch (e) {}
  try { sessionStorage.setItem(STORE_KEY, key); } catch (e) {}
}
function forgetKey() {
  try { localStorage.removeItem(STORE_KEY); } catch (e) {}
  try { sessionStorage.removeItem(STORE_KEY); } catch (e) {}
  keyInput.value = '';
  document.getElementById('keySource').textContent = '';
  document.getElementById('forget').hidden = true;
  document.getElementById('refresh').hidden = true;
  document.getElementById('content').innerHTML = 'Key forgotten. Paste the management key to load the pool.';
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
function statusBadge(status) {
  if (!status) return '';
  return ' <span class="badge ' + (status === 'active' ? 'ok' : 'bad') + '">' + esc(status) + '</span>';
}

// windowsEnforced returns windowLimits.limited, which is CommandCode's own
// plan/non-plan discriminator: subscription plans enforce rolling 5h/weekly
// credit-value windows, while pay-as-you-go (Provider) accounts do not and are
// limited by a prepaid balance instead. A null result means the credits
// endpoint has not reported the flag yet; the planId is then used as a fallback
// and the tooltip says so, so a guess is never presented as fact.
function windowsEnforced(a) {
  var bal = a.balance || {};
  if (bal.limited === true) return true;
  if (bal.limited === false) return false;
  var plan = a.plan || {};
  if (plan.id === 'individual-provider') return false;
  if (plan.known) return true;
  return null;
}
function isPayAsYouGo(a) { return windowsEnforced(a) === false; }
function enforcedLabel(a) {
  var v = windowsEnforced(a);
  if (v === true) return 'yes (rolling 5h/weekly credit-value limits)';
  if (v === false) return 'no (pay-as-you-go credit balance)';
  return 'unknown — no credits reading yet, inferred from planId';
}

function accountCell(a) {
  var plan = a.plan || {};
  var id = a.identity || {};
  var badges = '';
  if (a.disabled) badges += ' <span class="badge muted">disabled</span>';
  if (a.stale) badges += ' <span class="badge warn">stale</span>';
  if (plan.shared_keys > 1) badges += ' <span class="badge warn">shared pool</span>';
  var text = '<b>' + esc(a.name) + '</b>' + badges
    + (a.key_suffix ? '<br><span class="muted">…' + esc(a.key_suffix) + '</span>' : '');
  var tip = join([
    'providers: ' + ((a.providers && a.providers.length) ? a.providers.join(', ') : '-'),
    'user: ' + ([id.user_name, id.name].filter(Boolean).join(' / ') || '-'),
    id.email ? 'email: ' + id.email : '',
    id.org_id ? 'org: ' + id.org_id : '',
    'kind: ' + (isPayAsYouGo(a) ? 'pay-as-you-go (prepaid credit balance)' : 'subscription (rolling windows)'),
    'windows enforced: ' + enforcedLabel(a),
    'disabled: ' + (a.disabled ? 'yes' : 'no'),
    'stale: ' + (a.stale ? 'yes' : 'no'),
    'refreshed: ' + (a.refreshed_at ? fmtTime(a.refreshed_at) : 'never'),
    (a.data_age_seconds !== undefined && a.data_age_seconds !== null) ? 'data age: ' + a.data_age_seconds + 's' : '',
    a.attempted_at ? 'last attempt: ' + fmtTime(a.attempted_at) : '',
  ]);
  return { text: text, tip: tip };
}

function planCell(a) {
  var plan = a.plan || {};
  if (!plan.id) {
    return {
      text: '<span class="muted">no subscription</span>',
      tip: 'no active subscription reported by /alpha/billing/subscriptions',
    };
  }
  var text = '<b>' + esc(plan.name || plan.id) + '</b>' + statusBadge(plan.status);
  var tip = join([
    'planId: ' + plan.id,
    plan.marketing_name ? 'marketing name: ' + plan.marketing_name : '',
    plan.known === false ? 'catalog: UNKNOWN planId — raw API values only' : 'catalog: known plan',
    'status: ' + (plan.status || '-'),
    plan.quantity !== undefined ? 'quantity: ' + plan.quantity : '',
    'cancel at period end: ' + (plan.cancel_at_period_end ? 'yes' : 'no'),
    plan.subscription_id ? 'subscription: ' + plan.subscription_id : '',
    plan.price_id ? 'price: ' + plan.price_id : '',
    plan.created_at ? 'created: ' + fmtTime(plan.created_at) : '',
    plan.current_period_start ? 'billing period start: ' + fmtTime(plan.current_period_start) : '',
    plan.current_period_end ? 'billing period end: ' + fmtTime(plan.current_period_end) : '',
    (plan.days_remaining === undefined || plan.days_remaining === null) ? '' : 'days remaining: ' + plan.days_remaining,
    plan.pending_phase ? 'pending phase: ' + JSON.stringify(plan.pending_phase) : '',
  ]);
  return { text: text, tip: tip };
}

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
    'percent: ' + fmtNum(pct, 2) + '%  (credit value used vs cap)',
    'exceeded: ' + fmtBool(w.exceeded),
    'blocked: ' + (w.blocked ? 'yes' : 'no') + (w.blocked ? '  (effective: ' + (w.blocked_effective ? 'yes' : 'no — prepaid credits bypass caps') + ')' : ''),
    'reset at: ' + (w.reset_at ? fmtTime(w.reset_at) : 'unknown'),
    'resets in: ' + (w.resets_in || 'unknown'),
  ]);
  return { text: text, tip: tip };
}

function creditsCell(a) {
  var bal = a.balance || {};
  if (bal.total_remaining === undefined && bal.monthly_remaining === undefined) {
    return { text: '<span class="muted">n/a</span>', tip: 'no /alpha/billing/credits reading yet' };
  }
  var text = '<span class="mono">$' + fmtNum(bal.total_remaining, 2) + '</span>';
  var parts = [];
  if (bal.purchased_credits) parts.push('topped up $' + fmtNum(bal.purchased_credits, 2));
  if (bal.free_credits) parts.push('free $' + fmtNum(bal.free_credits, 2));
  if (parts.length) text += '<br><span class="muted">' + parts.join(' · ') + '</span>';
  var tip = join([
    'pay-as-you-go balance: $' + fmtNum(bal.total_remaining, 6),
    'purchased credits: $' + fmtNum(bal.purchased_credits, 6),
    'free credits: $' + fmtNum(bal.free_credits, 6),
    bal.monthly_remaining ? 'included monthly credits: $' + fmtNum(bal.monthly_remaining, 6) : '',
    'windows enforced: ' + enforcedLabel(a),
    'window exceeded flag: ' + fmtBool(bal.window_exceeded),
    'low-credit warning: ' + (bal.below_threshold ? 'ACTIVE (threshold $' + fmtNum(bal.credit_threshold, 2) + ')' : 'off'),
  ]);
  return { text: text, tip: tip };
}

function periodCell(a) {
  var plan = a.plan || {};
  if (!plan.current_period_start) {
    return { text: '<span class="muted">n/a</span>', tip: 'no billing period reported' };
  }
  var dr = plan.days_remaining;
  var text = '<span class="mono">' + (dr === undefined || dr === null ? '-' : dr + 'd') + '</span>'
    + '<br>' + bar(plan.period_elapsed_percent) + '<span class="muted">' + fmtNum(plan.period_elapsed_percent, 0) + '%</span>';
  var tip = join([
    'billing period start: ' + fmtTime(plan.current_period_start),
    'billing period end:   ' + fmtTime(plan.current_period_end),
    'days remaining: ' + (dr === undefined || dr === null ? '-' : dr),
    'period elapsed: ' + fmtNum(plan.period_elapsed_percent, 2) + '%',
  ]);
  return { text: text, tip: tip };
}

function spendCell(a) {
  var s = a.period_summary;
  if (!s) {
    return { text: '<span class="muted">n/a</span>', tip: 'billing-period spend unavailable or disabled (include-usage-summary)' };
  }
  var text = '<span class="mono">$' + fmtNum(s.total_cost, 4) + '</span>';
  var tip = join([
    'billing-period spend: $' + fmtNum(s.total_cost, 6),
    'credits charged: $' + fmtNum(s.total_credits, 6) + '  (monthly $' + fmtNum(s.total_monthly_credits, 6)
      + ', purchased $' + fmtNum(s.total_purchased_credits, 6) + ', free $' + fmtNum(s.total_free_credits, 6) + ')',
    'average per request: $' + fmtNum(s.average_cost, 6),
    'requested since: ' + (s.since || '-'),
    'effective since: ' + (s.since_effective || '-') + '   [' + (s.granularity || 'unknown') + ' bucketing]',
    'period basis: ' + (s.period_basis || '-'),
    '--- diagnostics (not billing units) ---',
    fmtNum(s.requests, 0) + ' requests, ' + fmtNum(s.failed, 0) + ' failed, success rate ' + fmtNum(s.success_rate, 2) + '%',
    'tokens: ' + fmtNum(s.tokens_in, 0) + ' in / ' + fmtNum(s.tokens_out, 0) + ' out / ' + fmtNum(s.tokens_total, 0) + ' total',
  ]);
  return { text: text, tip: tip };
}

function healthCell(a) {
  var errs = a.errors ? Object.keys(a.errors) : [];
  var text = errs.length ? '<span class="bad">' + errs.length + ' err</span>' : '<span class="ok">ok</span>';
  var tip = errs.length ? join(errs.map(function (k) { return k + ': ' + a.errors[k]; })) : 'all /alpha/* endpoints healthy';
  tip += '\nrefreshed: ' + (a.refreshed_at ? fmtTime(a.refreshed_at) : 'never');
  if (a.data_age_seconds !== undefined && a.data_age_seconds !== null) tip += '\ndata age: ' + a.data_age_seconds + 's';
  if (a.stale) tip += '\nSTALE: older than usage-stale-after';
  return { text: text, tip: tip };
}

function accountRow(a, payg) {
  var acct = accountCell(a);
  var plan = planCell(a);
  var spend = spendCell(a);
  var health = healthCell(a);
  var html = '<tr>'
    + cell(acct.text, acct.tip)
    + cell(plan.text, plan.tip);
  if (payg) {
    var credits = creditsCell(a);
    html += cell(credits.text, credits.tip, 'tip');
  } else {
    var w5 = windowCell((a.windows || {})['5h']);
    var ww = windowCell((a.windows || {}).weekly);
    var period = periodCell(a);
    html += cell(w5.text, '5-hour window\n' + w5.tip, 'tip');
    html += cell(ww.text, 'weekly window\n' + ww.tip, 'tip');
    html += cell(period.text, period.tip, 'tip');
  }
  return html + cell(spend.text, spend.tip, 'tip') + cell(health.text, health.tip, 'tip') + '</tr>';
}

function tableHTML(accounts, columns, payg) {
  var html = '<div class="card"><table class="grid"><tr>';
  for (var i = 0; i < columns.length; i++) html += '<th>' + columns[i] + '</th>';
  html += '</tr>';
  for (var j = 0; j < accounts.length; j++) html += accountRow(accounts[j], payg);
  return html + '</table></div>';
}

function render(status) {
  state.warn = status.warn_percent || 80;
  state.critical = status.critical_percent || 95;
  document.getElementById('meta').textContent =
    'v' + status.version + ' · ' + status.api_base_url
    + ' · refresh ' + status.refresh_interval
    + ' · stale after ' + status.stale_after
    + ' · ' + fmtTime(status.generated_at);
  document.getElementById('refresh').hidden = !keyInput.value.trim();
  document.getElementById('forget').hidden = !keyInput.value.trim();

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

  var subscription = [], payAsYouGo = [];
  for (var j = 0; j < status.accounts.length; j++) {
    if (isPayAsYouGo(status.accounts[j])) payAsYouGo.push(status.accounts[j]);
    else subscription.push(status.accounts[j]);
  }
  if (subscription.length) {
    html += '<div class="muted" style="margin:0 0 6px 2px">Windows enforced (<span class="mono">windowLimits.limited = true</span>) — subscription plans, throttled by rolling 5h/weekly credit-value limits</div>';
    html += tableHTML(subscription, ['Account', 'Plan', '5h', 'Weekly', 'Period', 'Spend', 'Health'], false);
  }
  if (payAsYouGo.length) {
    html += '<div class="muted" style="margin:12px 0 6px 2px">Windows not enforced (<span class="mono">windowLimits.limited = false</span>) — pay-as-you-go, limited by the prepaid credit balance</div>';
    html += tableHTML(payAsYouGo, ['Account', 'Plan', 'Credits', 'Spend', 'Health'], true);
  }
  html += '<div class="muted" style="margin-top:8px">Hover any cell for the full reading. '
    + 'CommandCode limits are credit/USD-equivalent, not request quotas, so every column is amount-based; '
    + 'request counts appear only as tooltip diagnostics.</div>';
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
  if (!key) {
    err.textContent = 'management key required — paste it once, it is remembered on this browser';
    keyInput.focus();
    return;
  }
  var status = await fetchJSON(MGMT + '/status', { headers: authHeaders() });
  rememberKey(key);
  document.getElementById('keySource').textContent = 'key accepted and remembered on this browser';
  render(status);
}

async function plansHTML() {
  var data = await fetchJSON(MGMT + '/plans', { headers: authHeaders() });
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

function showError(e) { document.getElementById('error').textContent = String(e); }
function runLoad() { load().catch(function (e) { showError(e); }); }

document.getElementById('load').addEventListener('click', runLoad);
document.getElementById('forget').addEventListener('click', forgetKey);
document.getElementById('refresh').addEventListener('click', async function () {
  try {
    await fetchJSON(MGMT + '/refresh', { method: 'POST', headers: authHeaders(), body: '{}' });
    setTimeout(runLoad, 1500);
  } catch (e) { showError(e); }
});
document.getElementById('togglePlans').addEventListener('click', async function () {
  var el = document.getElementById('plans');
  if (el.hidden) {
    try { el.innerHTML = await plansHTML(); el.hidden = false; }
    catch (e) { showError(e); }
  } else {
    el.hidden = true;
  }
});

// No interaction required: loading starts as soon as a key is available, and a
// key that is typed or pasted in triggers a load by itself.
var inputTimer = null;
keyInput.addEventListener('input', function () {
  if (inputTimer) { clearTimeout(inputTimer); inputTimer = null; }
  var value = keyInput.value.trim();
  if (!value) return;
  inputTimer = setTimeout(function () {
    if (keyInput.value.trim() === value) runLoad();
  }, 600);
});
keyInput.addEventListener('keydown', function (e) {
  if (e.key === 'Enter') {
    if (inputTimer) { clearTimeout(inputTimer); inputTimer = null; }
    runLoad();
  }
});
keyInput.addEventListener('blur', function () {
  if (inputTimer) { clearTimeout(inputTimer); inputTimer = null; }
});

var stored = readStoredKey();
if (stored) {
  keyInput.value = stored.key;
  document.getElementById('keySource').textContent = 'key loaded automatically from ' + stored.source;
  document.getElementById('refresh').hidden = false;
  document.getElementById('forget').hidden = false;
  runLoad();
} else {
  keyInput.focus();
  document.getElementById('content').innerHTML =
    'Paste the CPA management key once — it is remembered on this browser and the pool loads automatically from then on.';
}
</script>
</body>
</html>`
