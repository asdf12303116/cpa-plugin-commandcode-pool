package main

// statusPageHTML is the unauthenticated resource-page shell. It contains no
// account data; every data load goes through the management-key-gated API with
// the key the user enters in the browser (or that CPA Manager Plus persisted).
const statusPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CommandCode Pool</title>
<style>
:root { color-scheme: light dark; font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif; }
body { margin: 0; padding: 24px; background: Canvas; color: CanvasText; font-size: 14px; }
h1 { font-size: 20px; margin: 0 0 4px; }
h2 { font-size: 15px; margin: 0 0 10px; }
.card { border: 1px solid color-mix(in srgb, CanvasText 18%, transparent); border-radius: 10px; padding: 16px; margin-bottom: 16px; }
.row { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; }
input[type=password] { padding: 6px 10px; border-radius: 6px; border: 1px solid color-mix(in srgb, CanvasText 25%, transparent); min-width: 240px; background: transparent; color: inherit; }
button { padding: 6px 14px; border-radius: 6px; border: 1px solid color-mix(in srgb, CanvasText 25%, transparent); background: transparent; color: inherit; cursor: pointer; }
button:hover { background: color-mix(in srgb, CanvasText 8%, transparent); }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 12px; }
.box { border: 1px solid color-mix(in srgb, CanvasText 12%, transparent); border-radius: 8px; padding: 10px 12px; }
.box h3 { font-size: 12px; text-transform: uppercase; letter-spacing: .04em; opacity: .6; margin: 0 0 8px; }
.kv { display: flex; justify-content: space-between; gap: 8px; padding: 2px 0; }
.kv span:first-child { opacity: .65; }
.bar { display: inline-block; width: 100%; height: 8px; border-radius: 4px; background: color-mix(in srgb, CanvasText 12%, transparent); overflow: hidden; }
.bar > i { display: block; height: 100%; background: #16a34a; }
.bar > i.warn { background: #d97706; } .bar > i.bad { background: #dc2626; }
.ok { color: #16a34a; } .bad { color: #dc2626; } .warn { color: #d97706; }
.muted { opacity: .65; font-size: 12px; }
table { border-collapse: collapse; width: 100%; font-size: 13px; }
th, td { text-align: left; padding: 6px 10px; border-bottom: 1px solid color-mix(in srgb, CanvasText 12%, transparent); vertical-align: top; }
.badge { display: inline-block; padding: 1px 7px; border-radius: 999px; font-size: 11px; border: 1px solid currentColor; }
.acct-head { display: flex; flex-wrap: wrap; gap: 8px; align-items: baseline; margin-bottom: 6px; }
.acct-head b { font-size: 16px; }
#error { color: #dc2626; margin: 8px 0; min-height: 18px; }
</style>
</head>
<body>
<h1>CommandCode Pool</h1>
<div class="muted" id="meta"></div>
<div class="card">
  <div class="row">
    <label>CPA Management Key <input type="password" id="key" placeholder="management key" autocomplete="off"></label>
    <button id="load">Load</button>
    <button id="refresh">Refresh all</button>
    <button id="togglePlans">Plan catalog</button>
  </div>
  <div id="error"></div>
</div>
<div id="plans" style="display:none"></div>
<div id="content" class="muted">Enter the management key and press Load.</div>
<script>
var BASE = '/v0/management/plugins/commandcode-pool';
var keyInput = document.getElementById('key');
var state = { warn: 80, critical: 95 };

function headers() {
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
      if (st && typeof st.managementKey === 'string' && st.managementKey) return { key: st.managementKey, source: 'cpamp' };
    }
  } catch (e) {}
  try {
    var legacy = localStorage.getItem('managementKey');
    if (legacy) return { key: legacy, source: 'cpamp' };
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
function kv(k, v) { return '<div class="kv"><span>' + k + '</span><span>' + v + '</span></div>'; }

function windowBox(title, w) {
  var html = '<div class="box"><h3>' + title + ' window</h3>';
  if (!w || w.used === undefined) return html + '<div class="muted">no data</div></div>';
  html += kv('used', '$' + fmtNum(w.used, 4) + ' / ' + (w.cap === undefined ? '-' : '$' + fmtNum(w.cap, 2)));
  html += '<div style="margin:6px 0">' + bar(w.used_percent) + '</div>';
  html += kv('percent', w.used_percent === undefined ? '-' : fmtNum(w.used_percent, 2) + '%');
  html += kv('headroom', w.headroom === undefined ? '-' : '$' + fmtNum(w.headroom, 4));
  html += kv('resets', w.reset_at ? fmtTime(w.reset_at) + ' <span class="muted">(' + esc(w.resets_in) + ')</span>' : '-');
  if (w.blocked) {
    html += '<div class="' + (w.blocked_effective ? 'bad' : 'warn') + '">' + (w.blocked_effective ? 'window cap reached' : 'cap reached; prepaid credits bypass it') + '</div>';
  }
  return html + '</div>';
}

function accountCard(a) {
  var plan = a.plan || {};
  var bal = a.balance || {};
  var badge = '';
  if (a.disabled) badge += '<span class="badge muted">disabled</span>';
  if (a.stale) badge += '<span class="badge warn">stale</span>';
  if (plan.status) badge += '<span class="badge ' + (plan.status === 'active' ? 'ok' : 'bad') + '">' + esc(plan.status) + '</span>';
  if (plan.cancel_at_period_end) badge += '<span class="badge warn">cancels at period end</span>';
  if (plan.shared_keys > 1) badge += '<span class="badge warn">shared credit pool</span>';

  var html = '<div class="card"><div class="acct-head"><b>' + esc(a.name) + '</b><span class="muted">…' + esc(a.key_suffix) + '</span>' + badge + '</div>';

  var idParts = [];
  if (a.identity) {
    if (a.identity.user_name) idParts.push(esc(a.identity.user_name));
    if (a.identity.name) idParts.push(esc(a.identity.name));
    if (a.identity.email) idParts.push(esc(a.identity.email));
    if (a.identity.org_id) idParts.push('org ' + esc(a.identity.org_id));
  }
  if (idParts.length) html += '<div class="muted">' + idParts.join(' · ') + '</div>';
  if (a.providers && a.providers.length) html += '<div class="muted">' + esc(a.providers.join(', ')) + '</div>';

  html += '<div class="grid" style="margin-top:12px">';
  var planHtml = '<div class="box"><h3>Plan</h3>';
  planHtml += kv('plan', plan.id ? esc(plan.name || plan.id) + ' <span class="muted">(' + esc(plan.id) + ')</span>' : '<span class="muted">no subscription</span>');
  if (plan.id && plan.known === false) planHtml += '<div class="warn">unknown planId; showing raw values only</div>';
  if (plan.status) planHtml += kv('status', esc(plan.status));
  if (plan.current_period_start) planHtml += kv('period', fmtTime(plan.current_period_start) + ' → ' + fmtTime(plan.current_period_end));
  if (plan.days_remaining !== undefined && plan.days_remaining !== null) planHtml += kv('days left', plan.days_remaining);
  if (plan.period_elapsed_percent !== undefined && plan.period_elapsed_percent !== null) {
    planHtml += '<div style="margin:6px 0">' + bar(plan.period_elapsed_percent) + '</div>';
    planHtml += kv('elapsed', fmtNum(plan.period_elapsed_percent, 1) + '%');
  }
  if (plan.subscription_id) planHtml += kv('subscription', '<span class="muted">' + esc(plan.subscription_id) + '</span>');
  html += planHtml + '</div>';

  var balHtml = '<div class="box"><h3>Credits</h3>';
  if (bal.monthly_remaining !== undefined) {
    balHtml += kv('monthly remaining', '$' + fmtNum(bal.monthly_remaining, 4));
    if (bal.monthly_included !== undefined) balHtml += kv('included', '$' + fmtNum(bal.monthly_included, 2));
    if (bal.monthly_consumed !== undefined) balHtml += kv('consumed', '$' + fmtNum(bal.monthly_consumed, 4));
    if (bal.monthly_consumed_percent !== undefined) {
      balHtml += '<div style="margin:6px 0">' + bar(bal.monthly_consumed_percent) + '</div>';
      balHtml += kv('consumed', fmtNum(bal.monthly_consumed_percent, 1) + '%');
    }
  } else {
    balHtml += '<div class="muted">no data</div>';
  }
  if (bal.purchased_credits !== undefined) balHtml += kv('purchased', '$' + fmtNum(bal.purchased_credits, 4));
  if (bal.free_credits !== undefined) balHtml += kv('free', '$' + fmtNum(bal.free_credits, 4));
  if (bal.total_remaining !== undefined) balHtml += kv('total remaining', '$' + fmtNum(bal.total_remaining, 4));
  if (bal.limited !== undefined) balHtml += kv('windows enforced', bal.limited ? 'yes' : 'no');
  if (bal.below_threshold) balHtml += '<div class="warn">below credit threshold</div>';
  html += balHtml + '</div>';
  html += '</div>';

  html += '<div class="grid" style="margin-top:12px">';
  html += windowBox('5-hour', (a.windows || {})['5h']);
  html += windowBox('Weekly', (a.windows || {}).weekly);
  html += '</div>';

  if (a.period_summary) {
    var s = a.period_summary;
    html += '<div class="box" style="margin-top:12px"><h3>Billing-period usage</h3><div class="grid">';
    html += '<div>' + kv('requests', fmtNum(s.requests, 0)) + kv('completed', fmtNum(s.completed, 0)) + kv('failed', fmtNum(s.failed, 0)) + kv('success rate', fmtNum(s.success_rate, 2) + '%') + '</div>';
    html += '<div>' + kv('tokens in', fmtNum(s.tokens_in, 0)) + kv('tokens out', fmtNum(s.tokens_out, 0)) + kv('tokens total', fmtNum(s.tokens_total, 0)) + '</div>';
    html += '<div>' + kv('cost', '$' + fmtNum(s.total_cost, 4)) + kv('avg cost', '$' + fmtNum(s.average_cost, 6)) + kv('since', s.since ? fmtTime(s.since) : '-') + (s.period_basis ? kv('basis', esc(s.period_basis)) : '') + '</div>';
    html += '</div></div>';
  }

  if (a.errors) {
    var keys = Object.keys(a.errors);
    if (keys.length) {
      html += '<div class="warn" style="margin-top:10px">';
      for (var i = 0; i < keys.length; i++) html += '<div>' + esc(keys[i]) + ': ' + esc(a.errors[keys[i]]) + '</div>';
      html += '</div>';
    }
  }

  var foot = [];
  if (a.refreshed_at) foot.push('refreshed ' + fmtTime(a.refreshed_at));
  if (a.data_age_seconds !== undefined && a.data_age_seconds !== null) foot.push(a.data_age_seconds + 's old');
  if (a.attempted_at) foot.push('last attempt ' + fmtTime(a.attempted_at));
  if (foot.length) html += '<div class="muted" style="margin-top:8px">' + foot.join(' · ') + '</div>';

  return html + '</div>';
}

async function api(path, options) {
  var resp = await fetch(BASE + path, Object.assign({ headers: headers() }, options || {}));
  if (!resp.ok && resp.status !== 202) throw new Error('HTTP ' + resp.status + (resp.status === 401 ? ' (bad management key?)' : ''));
  return resp.json();
}

async function load() {
  document.getElementById('error').textContent = '';
  try {
    try { sessionStorage.setItem('ccp-key', keyInput.value.trim()); } catch (e) {}
    var data = await api('/status');
    state.warn = data.warn_percent || 80;
    state.critical = data.critical_percent || 95;
    document.getElementById('meta').textContent = 'v' + data.version + ' · ' + data.api_base_url + ' · refresh ' + data.refresh_interval + ' · stale after ' + data.stale_after + ' · ' + fmtTime(data.generated_at);

    var html = '';
    if (data.config_error) html += '<div class="card bad">config error: ' + esc(data.config_error) + '</div>';
    var shared = data.shared_subscriptions || [];
    for (var i = 0; i < shared.length; i++) {
      var g = shared[i];
      html += '<div class="card warn">Shared credit pool: ' + esc(g.accounts.join(', ')) + ' share subscription ' + esc(g.subscription_id) + ' (' + esc(g.plan_id || 'unknown plan') + ') — one balance for all of them.</div>';
    }
    if (!data.accounts.length) {
      html += '<div class="card muted">No CommandCode API keys discovered. Check cpa-config-path and that a credential base URL contains commandcode.ai.</div>';
    }
    for (var j = 0; j < data.accounts.length; j++) html += accountCard(data.accounts[j]);
    document.getElementById('content').innerHTML = html;
  } catch (err) {
    document.getElementById('error').textContent = String(err);
  }
}

async function plansHTML() {
  var data = await api('/plans');
  var html = '<div class="card"><h2>Plan catalog</h2><table><tr><th>planId</th><th>Name</th><th>Marketing</th><th>Monthly credits</th><th>5h cap</th><th>Weekly cap</th></tr>';
  for (var i = 0; i < data.plans.length; i++) {
    var p = data.plans[i];
    html += '<tr><td>' + esc(p.id) + '</td><td>' + esc(p.name) + '</td><td>' + esc(p.marketing_name || '') + '</td>';
    html += '<td>' + (p.includes_credits ? '$' + fmtNum(p.monthly_credits, 2) : 'pay-as-you-go') + '</td>';
    html += '<td>' + (p.five_hour_cap === undefined ? '-' : '$' + fmtNum(p.five_hour_cap, 2)) + '</td>';
    html += '<td>' + (p.weekly_cap === undefined ? '-' : '$' + fmtNum(p.weekly_cap, 2)) + '</td></tr>';
  }
  html += '</table><div class="muted" style="margin-top:8px">Reference data from the CommandCode CLI bundle and pricing page; the live API remains authoritative for caps and balances.</div></div>';
  return html;
}

document.getElementById('load').addEventListener('click', load);
document.getElementById('refresh').addEventListener('click', async function () {
  try { await api('/refresh', { method: 'POST', body: '{}' }); setTimeout(load, 1500); }
  catch (err) { document.getElementById('error').textContent = String(err); }
});
document.getElementById('togglePlans').addEventListener('click', async function () {
  var el = document.getElementById('plans');
  if (!el.style.display || el.style.display === 'none') {
    try { el.innerHTML = await plansHTML(); el.style.display = 'block'; }
    catch (err) { document.getElementById('error').textContent = String(err); }
  } else {
    el.style.display = 'none';
  }
});
keyInput.addEventListener('keydown', function (e) { if (e.key === 'Enter') load(); });

try { keyInput.value = sessionStorage.getItem('ccp-key') || ''; } catch (e) {}
if (keyInput.value) {
  load();
} else {
  var stored = readStoredKey();
  if (stored && stored.source === 'cpamp') {
    keyInput.value = stored.key;
    document.getElementById('meta').textContent = 'management key loaded from CPA Manager Plus';
    load();
  }
}
</script>
</body>
</html>`
