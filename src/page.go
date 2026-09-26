package main

// statusPageHTML is the plugin resource page.
//
// The page renders one compact table row per account; clicking a row expands an
// inline detail panel with the full reading, so no hover interaction is needed.
// It loads by itself on open: the management key is taken from CPA Manager Plus'
// persisted auth store, from this page's own remembered value, or from the
// current tab, and a key typed or pasted into the field triggers the load
// automatically, so the Load button is only a fallback.
//
// Styling follows CPA Manager Plus (seakee/CPA-Manager-Plus): cpamp embeds
// plugin resource pages in an iframe and injects the host design tokens into it
// (html[data-cpamp-plugin-host] + data-theme + every host custom property), so
// this page never defines host token names itself. It only reads them through
// --ccp-* with cpamp's own light palette as the fallback, which keeps it
// identical to cpamp when hosted and usable standalone.
//
// CommandCode throttles on credit/USD-equivalent value rather than request
// quotas, so every column is amount-based. Columns also depend on
// windowLimits.limited, the API's own plan/non-plan discriminator: subscription
// plans are limited by the rolling 5h/weekly windows and do not need credit or
// spend columns, while pay-as-you-go (Provider) accounts have no windows and are
// limited by their prepaid balance.
const statusPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CommandCode Pool</title>
<style>
/* ---------------------------------------------------------------------------
   cpamp design tokens, consumed with cpamp's light palette as the fallback.
   --------------------------------------------------------------------------- */
:root {
  --ccp-font: var(--cpamp-plugin-font-family, Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif);
  --ccp-mono: var(--font-family-mono, var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace));
  --ccp-bg: var(--bg-primary, #ffffff);
  --ccp-page-bg: var(--app-bg, #eff2f7);
  --ccp-muted: var(--bg-tertiary, #f6faff);
  --ccp-border: var(--border-color, rgba(15, 23, 42, 0.08));
  --ccp-border-strong: var(--app-border-strong, rgba(15, 23, 42, 0.12));
  --ccp-text: var(--text-primary, #2c3e50);
  --ccp-text-2: var(--text-secondary, #5f6c7b);
  --ccp-text-3: var(--text-tertiary, #8b95a6);
  --ccp-hover: var(--bg-hover, rgba(59, 130, 246, 0.08));
  --ccp-primary: var(--primary-color, #409eff);
  --ccp-primary-hover: var(--primary-hover, #79bbff);
  --ccp-radius-sm: var(--app-radius-sm, 8px);
  --ccp-radius-md: var(--app-radius-md, 12px);
  --ccp-shadow: var(--shadow-lg, none);
  --ccp-track: var(--data-track-bg, rgba(95, 108, 123, 0.16));
  --ccp-green: var(--data-green-base, #22c55e);
  --ccp-amber: var(--data-amber-base, #f59e0b);
  --ccp-red: var(--data-red-base, #ef4444);
  --ccp-badge-ok-bg: var(--data-badge-success-bg, #f0fdf4);
  --ccp-badge-ok-border: var(--data-badge-success-border, #bbf7d0);
  --ccp-badge-ok-text: var(--data-badge-success-text, #15803d);
  --ccp-badge-warn-bg: var(--data-badge-warning-bg, #fffbeb);
  --ccp-badge-warn-border: var(--data-badge-warning-border, #fde68a);
  --ccp-badge-warn-text: var(--data-badge-warning-text, #b45309);
  --ccp-badge-bad-bg: var(--data-badge-danger-bg, #fef2f2);
  --ccp-badge-bad-border: var(--data-badge-danger-border, #fecaca);
  --ccp-badge-bad-text: var(--data-badge-danger-text, #b91c1c);
  --ccp-badge-info-bg: var(--data-badge-info-bg, #eff6ff);
  --ccp-badge-info-border: var(--data-badge-info-border, #bfdbfe);
  --ccp-badge-info-text: var(--data-badge-info-text, #1d4ed8);
  --ccp-badge-muted-bg: var(--data-badge-neutral-bg, #f8fafc);
  --ccp-badge-muted-border: var(--data-badge-neutral-border, #cbd5e1);
  --ccp-badge-muted-text: var(--data-badge-neutral-text, #475569);
}

/* Standalone dark palette (cpamp's dark values); skipped while cpamp hosts us
   because it already injected the real theme variables. */
@media (prefers-color-scheme: dark) {
  html:not([data-cpamp-plugin-host]) {
    --ccp-bg: rgba(24, 28, 40, 0.9);
    --ccp-page-bg: #0a0a0a;
    --ccp-muted: rgba(255, 255, 255, 0.06);
    --ccp-border: rgba(255, 255, 255, 0.08);
    --ccp-border-strong: rgba(255, 255, 255, 0.12);
    --ccp-text: #e5e5e5;
    --ccp-text-2: #a3a3a3;
    --ccp-text-3: #7a7a7a;
    --ccp-hover: rgba(96, 165, 250, 0.18);
    --ccp-track: rgba(255, 255, 255, 0.08);
    --ccp-badge-ok-bg: rgba(74, 222, 128, 0.14);
    --ccp-badge-ok-border: rgba(74, 222, 128, 0.24);
    --ccp-badge-ok-text: #4ade80;
    --ccp-badge-warn-bg: rgba(251, 191, 36, 0.14);
    --ccp-badge-warn-border: rgba(251, 191, 36, 0.24);
    --ccp-badge-warn-text: #fbbf24;
    --ccp-badge-bad-bg: rgba(248, 113, 113, 0.14);
    --ccp-badge-bad-border: rgba(248, 113, 113, 0.24);
    --ccp-badge-bad-text: #f87171;
    --ccp-badge-info-bg: rgba(96, 165, 250, 0.14);
    --ccp-badge-info-border: rgba(96, 165, 250, 0.24);
    --ccp-badge-info-text: #60a5fa;
    --ccp-badge-muted-bg: rgba(148, 163, 184, 0.1);
    --ccp-badge-muted-border: rgba(148, 163, 184, 0.24);
    --ccp-badge-muted-text: #94a3b8;
  }
}

* { box-sizing: border-box; }

body {
  margin: 0;
  padding: 16px 18px 24px;
  background: var(--ccp-page-bg);
  color: var(--ccp-text);
  font-family: var(--ccp-font);
  font-size: 14px;
  line-height: 1.5;
}

.head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 10px; margin-bottom: 12px; }
h1 { margin: 0; font-size: 18px; font-weight: 700; letter-spacing: 0; }
h2 { margin: 0 0 10px; font-size: 14px; font-weight: 700; }

.card {
  margin-bottom: 12px;
  padding: 12px;
  border: 1px solid var(--ccp-border);
  border-radius: var(--ccp-radius-md);
  background: var(--ccp-bg);
  box-shadow: var(--ccp-shadow);
}
.row { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }

label { color: var(--ccp-text-2); font-size: 12px; }
input[type=password] {
  min-height: 34px;
  margin-left: 6px;
  padding: 0 10px;
  border: 1px solid var(--ccp-border);
  border-radius: var(--ccp-radius-sm);
  background: var(--ccp-muted);
  color: var(--ccp-text);
  font-family: inherit;
  font-size: 13px;
  outline: none;
}
input[type=password]:focus { border-color: var(--ccp-primary); }

button {
  min-height: 34px;
  padding: 0 12px;
  border: 1px solid var(--ccp-border);
  border-radius: var(--ccp-radius-md);
  background: var(--ccp-muted);
  color: var(--ccp-text);
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.2;
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}
button:hover:not(:disabled) { border-color: var(--ccp-primary); background: var(--ccp-hover); color: var(--ccp-primary); }
button.primary { border-color: var(--ccp-primary); background: var(--ccp-primary); color: #ffffff; }
button.primary:hover { border-color: var(--ccp-primary-hover); background: var(--ccp-primary-hover); color: #ffffff; }

table.grid { width: 100%; border-collapse: collapse; font-size: 13px; }
table.grid th {
  padding: 8px 10px;
  background: color-mix(in srgb, var(--ccp-muted) 72%, var(--ccp-bg));
  color: var(--ccp-text-2);
  font-size: 12px;
  font-weight: 600;
  text-align: left;
  white-space: nowrap;
}
table.grid td {
  padding: 8px 10px;
  border-top: 1px solid var(--ccp-border);
  vertical-align: middle;
  white-space: nowrap;
}

tr.acct { cursor: pointer; }
tr.acct:hover td { background: var(--ccp-hover); }
tr.acct:focus-visible { outline: 2px solid var(--ccp-primary); outline-offset: -2px; }
.caret {
  display: inline-block;
  width: 11px;
  margin-right: 6px;
  color: var(--ccp-text-3);
  transition: transform 0.15s ease;
}
tr.acct.open .caret { transform: rotate(90deg); }

tr.detail > td {
  padding: 0;
  border-top: 0;
  background: color-mix(in srgb, var(--ccp-muted) 62%, var(--ccp-bg));
  white-space: normal;
}
.detail-body { padding: 12px 12px 14px 27px; }
.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 14px 20px;
}
.detail-section { min-width: 0; }
.detail-title { margin-bottom: 6px; color: var(--ccp-text-2); font-size: 12px; font-weight: 600; }
.kv { display: flex; justify-content: space-between; gap: 14px; padding: 2px 0; font-size: 12px; }
.kv > span:first-child { flex: 0 0 auto; color: var(--ccp-text-3); }
.kv > span:last-child { min-width: 0; color: var(--ccp-text); text-align: right; overflow-wrap: anywhere; }

.muted { color: var(--ccp-text-3); font-size: 12px; }
.mono { font-family: var(--ccp-mono); font-variant-numeric: tabular-nums; }
.ok { color: var(--ccp-green); }
.warn { color: var(--ccp-amber); }
.bad { color: var(--ccp-red); }
.section-label { margin: 14px 0 6px; color: var(--ccp-text-3); font-size: 12px; }
.section-label:first-child { margin-top: 0; }

.badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 8px;
  border: 1px solid transparent;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.5;
  white-space: nowrap;
}
.badge::before { content: ''; width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.badge.ok { background: var(--ccp-badge-ok-bg); border-color: var(--ccp-badge-ok-border); color: var(--ccp-badge-ok-text); }
.badge.warn { background: var(--ccp-badge-warn-bg); border-color: var(--ccp-badge-warn-border); color: var(--ccp-badge-warn-text); }
.badge.bad { background: var(--ccp-badge-bad-bg); border-color: var(--ccp-badge-bad-border); color: var(--ccp-badge-bad-text); }
.badge.info { background: var(--ccp-badge-info-bg); border-color: var(--ccp-badge-info-border); color: var(--ccp-badge-info-text); }
.badge.muted { background: var(--ccp-badge-muted-bg); border-color: var(--ccp-badge-muted-border); color: var(--ccp-badge-muted-text); }

.bar {
  display: inline-block;
  width: 56px;
  height: 8px;
  margin-right: 7px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--ccp-track);
  vertical-align: middle;
}
.bar > i { display: block; height: 100%; background: var(--ccp-green); transition: width 0.2s ease; }
.bar > i.warn { background: var(--ccp-amber); }
.bar > i.bad { background: var(--ccp-red); }

#error { margin-top: 8px; color: var(--ccp-red); font-size: 12px; min-height: 16px; }
</style>
</head>
<body>
<div class="head">
  <h1>CommandCode Pool</h1>
  <div class="muted" id="meta"></div>
</div>
<div class="card">
  <div class="row">
    <label>Management key
      <input type="password" id="key" placeholder="loaded automatically when available" autocomplete="off">
    </label>
    <button id="load" class="primary">Reload</button>
    <button id="refresh" hidden>Refresh all</button>
    <button id="togglePlans">Plan catalog</button>
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
var contentEl = document.getElementById('content');
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

/* ---- formatting ---- */

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
function joinList(values) {
  return values.filter(function (v) { return v !== '' && v !== undefined && v !== null; });
}
function statusBadge(status) {
  if (!status) return '';
  return ' <span class="badge ' + (status === 'active' ? 'ok' : 'bad') + '">' + esc(status) + '</span>';
}

/* ---- plan kind ---- */

// windowsEnforced returns windowLimits.limited, which is CommandCode's own
// plan/non-plan discriminator: subscription plans enforce rolling 5h/weekly
// credit-value windows, while pay-as-you-go (Provider) accounts do not and are
// limited by a prepaid balance instead. A null result means the credits
// endpoint has not reported the flag yet; the planId is then used as a fallback
// and the detail panel says so, so a guess is never presented as fact.
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

/* ---- cell builders: compact text plus the detail-panel entries ---- */

function accountCell(a) {
  var plan = a.plan || {};
  var id = a.identity || {};
  var badges = '';
  if (a.disabled) badges += ' <span class="badge muted">disabled</span>';
  if (a.stale) badges += ' <span class="badge warn">stale</span>';
  if (plan.shared_keys > 1) badges += ' <span class="badge info">shared pool</span>';
  return {
    text: '<b>' + esc(a.name) + '</b>' + badges
      + (a.key_suffix ? '<br><span class="muted">…' + esc(a.key_suffix) + '</span>' : ''),
    title: 'Account',
    items: [
      ['name', a.name],
      ['key suffix', a.key_suffix ? '…' + a.key_suffix : ''],
      ['providers', (a.providers && a.providers.length) ? a.providers.join(', ') : ''],
      ['user', joinList([id.user_name, id.name]).join(' / ')],
      ['email', id.email || ''],
      ['org id', id.org_id || ''],
      ['windows enforced', enforcedLabel(a)],
      ['disabled', a.disabled ? 'yes' : 'no'],
      ['stale', a.stale ? 'yes' : 'no'],
      ['refreshed', a.refreshed_at ? fmtTime(a.refreshed_at) : 'never'],
      ['data age', (a.data_age_seconds === undefined || a.data_age_seconds === null) ? '' : a.data_age_seconds + 's'],
      ['last attempt', a.attempted_at ? fmtTime(a.attempted_at) : ''],
    ],
    sections: [],
  };
}

function planCell(a) {
  var plan = a.plan || {};
  if (!plan.id) {
    return {
      text: '<span class="muted">no subscription</span>',
      title: 'Plan',
      items: [['subscription', 'none reported by /alpha/billing/subscriptions']],
      sections: [],
    };
  }
  return {
    text: '<b>' + esc(plan.name || plan.id) + '</b>' + statusBadge(plan.status),
    title: 'Plan',
    items: [
      ['planId', plan.id],
      ['name', plan.name || ''],
      ['marketing name', plan.marketing_name || ''],
      ['catalog', plan.known === false ? 'UNKNOWN planId — raw API values only' : 'known plan'],
      ['status', plan.status || ''],
      ['quantity', plan.quantity === undefined ? '' : String(plan.quantity)],
      ['cancel at period end', plan.cancel_at_period_end ? 'yes' : 'no'],
      ['subscription id', plan.subscription_id || ''],
      ['price id', plan.price_id || ''],
      ['created', plan.created_at ? fmtTime(plan.created_at) : ''],
      ['pending phase', plan.pending_phase ? JSON.stringify(plan.pending_phase) : ''],
    ],
    sections: [],
  };
}

function windowCell(title, w) {
  if (!w || w.used === undefined) {
    return { text: '<span class="muted">n/a</span>', title: title, items: [['reading', 'not available yet']], sections: [] };
  }
  var pct = w.used_percent;
  return {
    text: bar(pct) + '<span class="mono ' + pctClass(pct) + '">' + fmtNum(pct, 1) + '%</span>',
    title: title,
    items: [
      ['used', '$' + fmtNum(w.used, 6)],
      ['cap', '$' + fmtNum(w.cap, 2)],
      ['headroom', '$' + fmtNum(w.headroom, 6)],
      ['percent', fmtNum(pct, 2) + '% (credit value used vs cap)'],
      ['exceeded', fmtBool(w.exceeded)],
      ['blocked', w.blocked ? 'yes' : 'no'],
      ['blocked effective', w.blocked ? (w.blocked_effective ? 'yes' : 'no — prepaid credits bypass caps') : 'no'],
      ['reset at', w.reset_at ? fmtTime(w.reset_at) : 'unknown'],
      ['resets in', w.resets_in || 'unknown'],
    ],
    sections: [],
  };
}

function creditsCell(a) {
  var bal = a.balance || {};
  if (bal.total_remaining === undefined && bal.monthly_remaining === undefined) {
    return { text: '<span class="muted">n/a</span>', title: 'Credits', items: [['reading', 'not available yet']], sections: [] };
  }
  var text = '<span class="mono">$' + fmtNum(bal.total_remaining, 2) + '</span>';
  var parts = [];
  if (bal.purchased_credits) parts.push('topped up $' + fmtNum(bal.purchased_credits, 2));
  if (bal.free_credits) parts.push('free $' + fmtNum(bal.free_credits, 2));
  if (parts.length) text += '<br><span class="muted">' + parts.join(' · ') + '</span>';
  return {
    text: text,
    title: 'Credits',
    items: [
      ['balance', '$' + fmtNum(bal.total_remaining, 6)],
      ['purchased credits', '$' + fmtNum(bal.purchased_credits, 6)],
      ['free credits', '$' + fmtNum(bal.free_credits, 6)],
      ['monthly credits left', bal.monthly_remaining ? '$' + fmtNum(bal.monthly_remaining, 6) : ''],
      ['windows enforced', enforcedLabel(a)],
      ['window exceeded flag', fmtBool(bal.window_exceeded)],
      ['low-credit warning', bal.below_threshold ? 'ACTIVE (threshold $' + fmtNum(bal.credit_threshold, 2) + ')' : 'off'],
    ],
    sections: [],
  };
}

function periodCell(a) {
  var plan = a.plan || {};
  if (!plan.current_period_start) {
    return { text: '<span class="muted">n/a</span>', title: 'Billing period', items: [['period', 'not reported']], sections: [] };
  }
  var dr = plan.days_remaining;
  return {
    text: '<span class="mono">' + (dr === undefined || dr === null ? '-' : dr + 'd') + '</span>'
      + '<br>' + bar(plan.period_elapsed_percent) + '<span class="muted">' + fmtNum(plan.period_elapsed_percent, 0) + '%</span>',
    title: 'Billing period',
    items: [
      ['start', fmtTime(plan.current_period_start)],
      ['end', fmtTime(plan.current_period_end)],
      ['days remaining', (dr === undefined || dr === null) ? '' : String(dr)],
      ['elapsed', fmtNum(plan.period_elapsed_percent, 2) + '%'],
    ],
    sections: [],
  };
}

function spendCell(a) {
  var s = a.period_summary;
  if (!s) {
    return { text: '<span class="muted">n/a</span>', title: 'Billing-period spend', items: [['reading', 'unavailable or disabled (include-usage-summary)']], sections: [] };
  }
  var tip = {
    text: '<span class="mono">$' + fmtNum(s.total_cost, 4) + '</span>',
    title: 'Billing-period spend',
    items: [
      ['spend', '$' + fmtNum(s.total_cost, 6)],
      ['credits charged', '$' + fmtNum(s.total_credits, 6)],
      ['monthly credits', '$' + fmtNum(s.total_monthly_credits, 6)],
      ['purchased credits', '$' + fmtNum(s.total_purchased_credits, 6)],
      ['free credits', '$' + fmtNum(s.total_free_credits, 6)],
      ['average per request', '$' + fmtNum(s.average_cost, 6)],
      ['requested since', s.since || ''],
      ['effective since', (s.since_effective || '') + (s.granularity ? '  [' + s.granularity + ' bucketing]' : '')],
      ['period basis', s.period_basis || ''],
    ],
    sections: [{
      title: 'Diagnostics (not billing units)',
      items: [
        ['requests', fmtNum(s.requests, 0)],
        ['failed', fmtNum(s.failed, 0)],
        ['success rate', fmtNum(s.success_rate, 2) + '%'],
        ['tokens in', fmtNum(s.tokens_in, 0)],
        ['tokens out', fmtNum(s.tokens_out, 0)],
        ['tokens total', fmtNum(s.tokens_total, 0)],
      ],
    }],
  };
  return tip;
}

function healthCell(a) {
  var errs = a.errors ? Object.keys(a.errors) : [];
  var items = [['endpoints', errs.length ? errs.length + ' failing' : 'all /alpha/* endpoints healthy']];
  for (var i = 0; i < errs.length; i++) items.push([errs[i], a.errors[errs[i]]]);
  return {
    text: errs.length ? '<span class="bad">' + errs.length + ' err</span>' : '<span class="ok">ok</span>',
    title: 'Health',
    items: items,
    sections: [],
  };
}

/* ---- rendering ---- */

function renderSections(cells) {
  var html = '';
  for (var i = 0; i < cells.length; i++) {
    var cell = cells[i];
    var items = joinList(cell.items.map(function (entry) { return entry[1] ? entry : null; }));
    if (!items.length) continue;
    html += '<div class="detail-section"><div class="detail-title">' + esc(cell.title) + '</div>';
    for (var j = 0; j < items.length; j++) {
      html += '<div class="kv"><span>' + esc(items[j][0]) + '</span> <span class="mono">' + esc(items[j][1]) + '</span></div>';
    }
    html += '</div>';
    var sections = cell.sections || [];
    for (var k = 0; k < sections.length; k++) {
      var section = sections[k];
      var sectionItems = joinList(section.items.map(function (entry) { return entry[1] ? entry : null; }));
      if (!sectionItems.length) continue;
      html += '<div class="detail-section"><div class="detail-title">' + esc(section.title) + '</div>';
      for (var m = 0; m < sectionItems.length; m++) {
        html += '<div class="kv"><span>' + esc(sectionItems[m][0]) + '</span> <span class="mono">' + esc(sectionItems[m][1]) + '</span></div>';
      }
      html += '</div>';
    }
  }
  return html;
}

function accountRows(a, payg, index) {
  var cells = [accountCell(a), planCell(a)];
  if (payg) {
    cells.push(creditsCell(a));
    cells.push(spendCell(a));
  } else {
    cells.push(windowCell('5-hour window', (a.windows || {})['5h']));
    cells.push(windowCell('Weekly window', (a.windows || {}).weekly));
    cells.push(periodCell(a));
  }
  cells.push(healthCell(a));

  var columns = cells.length;
  var row = '<tr class="acct" data-idx="' + index + '" tabindex="0" role="button" aria-expanded="false">';
  for (var i = 0; i < cells.length; i++) {
    var prefix = i === 0 ? '<span class="caret">▸</span>' : '';
    row += '<td>' + prefix + cells[i].text + '</td>';
  }
  row += '</tr>';

  var detail = '<tr class="detail" data-detail="' + index + '" hidden><td colspan="' + columns + '">'
    + '<div class="detail-body"><div class="detail-grid">' + renderSections(cells) + '</div></div></td></tr>';
  return row + detail;
}

function tableHTML(accounts, columns, payg, startIndex) {
  var html = '<div class="card"><table class="grid"><thead><tr>';
  for (var i = 0; i < columns.length; i++) html += '<th>' + columns[i] + '</th>';
  html += '</tr></thead><tbody>';
  for (var j = 0; j < accounts.length; j++) html += accountRows(accounts[j], payg, startIndex + j);
  return html + '</tbody></table></div>';
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
    contentEl.innerHTML = html;
    return;
  }

  var subscription = [], payAsYouGo = [];
  for (var j = 0; j < status.accounts.length; j++) {
    if (isPayAsYouGo(status.accounts[j])) payAsYouGo.push(status.accounts[j]);
    else subscription.push(status.accounts[j]);
  }
  if (subscription.length) {
    html += '<div class="section-label">Windows enforced (<span class="mono">windowLimits.limited = true</span>) — subscription plans, throttled by rolling 5h/weekly credit-value limits</div>';
    html += tableHTML(subscription, ['Account', 'Plan', '5h', 'Weekly', 'Period', 'Health'], false, 0);
  }
  if (payAsYouGo.length) {
    html += '<div class="section-label">Windows not enforced (<span class="mono">windowLimits.limited = false</span>) — pay-as-you-go, limited by the prepaid credit balance</div>';
    html += tableHTML(payAsYouGo, ['Account', 'Plan', 'Credits', 'Spend', 'Health'], true, subscription.length);
  }
  html += '<div class="muted" style="margin-top:10px">Click a row to expand the full reading. '
    + 'CommandCode limits are credit/USD-equivalent, not request quotas, so every column is amount-based.</div>';
  contentEl.innerHTML = html;
}

function toggleRow(row) {
  var detail = contentEl.querySelector('tr.detail[data-detail="' + row.getAttribute('data-idx') + '"]');
  if (!detail) return;
  var open = detail.hidden;
  detail.hidden = !open;
  row.classList.toggle('open', open);
  row.setAttribute('aria-expanded', open ? 'true' : 'false');
}

contentEl.addEventListener('click', function (e) {
  var target = e.target;
  var row = target && target.closest ? target.closest('tr.acct') : null;
  if (row) toggleRow(row);
});
contentEl.addEventListener('keydown', function (e) {
  if (e.key !== 'Enter' && e.key !== ' ') return;
  var target = e.target;
  var row = target && target.closest ? target.closest('tr.acct') : null;
  if (!row) return;
  e.preventDefault();
  toggleRow(row);
});

/* ---- data ---- */

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
    err.textContent = 'management key required — paste it once, it is loaded automatically afterwards';
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
  var html = '<div class="card"><h2>Plan catalog</h2><table class="grid"><thead><tr><th>planId</th><th>Name</th><th>Marketing</th><th>Monthly credits</th><th>5h cap</th><th>Weekly cap</th></tr></thead><tbody>';
  for (var i = 0; i < data.plans.length; i++) {
    var p = data.plans[i];
    html += '<tr><td class="mono">' + esc(p.id) + '</td><td>' + esc(p.name) + '</td><td>' + esc(p.marketing_name || '')
      + '</td><td>' + (p.includes_credits ? '$' + fmtNum(p.monthly_credits, 2) : 'pay-as-you-go')
      + '</td><td>' + (p.five_hour_cap === undefined ? '-' : '$' + fmtNum(p.five_hour_cap, 2))
      + '</td><td>' + (p.weekly_cap === undefined ? '-' : '$' + fmtNum(p.weekly_cap, 2)) + '</td></tr>';
  }
  return html + '</tbody></table><div class="muted" style="margin-top:8px">Reference data from the CommandCode CLI bundle and pricing page; the live API remains authoritative for caps and balances.</div></div>';
}

function showError(e) { document.getElementById('error').textContent = String(e); }
function runLoad() { load().catch(function (e) { showError(e); }); }

document.getElementById('load').addEventListener('click', runLoad);
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
  runLoad();
} else {
  keyInput.focus();
  contentEl.innerHTML =
    'Paste the CPA management key once — it is remembered on this browser and the pool loads automatically from then on.';
}
</script>
</body>
</html>`
