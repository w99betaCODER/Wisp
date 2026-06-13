"use strict";

const state = { plans: [], orders: [] };

// --- tiny helpers ---------------------------------------------------------

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  if (res.status === 401) {
    document.getElementById("login").classList.remove("hidden");
    throw new Error("auth");
  }
  if (!res.ok) {
    let msg = res.status + " " + res.statusText;
    try { const j = await res.json(); if (j.error) msg = j.error; } catch (_) {}
    throw new Error(msg);
  }
  return res.status === 204 ? null : res.json();
}

// applyBranding white-labels the page from the /api/branding settings.
function applyBranding(b) {
  if (!b) return;
  if (b.name) {
    document.title = b.name + " — VPN panel";
    document.getElementById("brand-name").textContent = b.name;
    document.getElementById("login-name").textContent = b.name;
  }
  if (b.tagline) document.querySelector(".sub").textContent = b.tagline;
  if (b.accent) {
    const root = document.documentElement.style;
    root.setProperty("--accent", b.accent);
    root.setProperty("--accent-2", b.accent);
  }
}

async function doLogin(ev) {
  ev.preventDefault();
  try {
    await api("POST", "/api/login", { token: document.getElementById("login-token").value });
    document.getElementById("login").classList.add("hidden");
    toast("Signed in");
    load();
  } catch (e) {
    if (e.message !== "auth") toast(e.message, true);
  }
}

function toast(msg, isErr) {
  const t = document.getElementById("toast");
  t.textContent = msg;
  t.className = "toast show" + (isErr ? " err" : "");
  setTimeout(() => (t.className = "toast"), 2600);
}

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function fmtBytes(n) {
  if (!n) return "0 B";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return (n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)) + " " + u[i];
}

function fmtMoney(cents, currency) {
  return (cents / 100).toFixed(2) + " " + (currency || "USD");
}

function fmtDate(s) {
  if (!s) return "never";
  return new Date(s).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

const toggle = (id) => document.getElementById(id).classList.toggle("hidden");

function copySub(id) {
  const url = location.origin + "/sub/" + id;
  navigator.clipboard.writeText(url).then(
    () => toast("Subscription link copied"),
    () => toast(url, false)
  );
}

// --- rendering ------------------------------------------------------------

function usageCell(u) {
  if (!u.data_limit) {
    return `<div class="usage"><div class="nums">${fmtBytes(u.used)} / ∞</div>
      <div class="bar"><i style="width:0"></i></div></div>`;
  }
  const pct = Math.min(100, Math.round((u.used / u.data_limit) * 100));
  const cls = pct >= 100 ? "full" : pct >= 80 ? "warn" : "";
  return `<div class="usage"><div class="nums">${fmtBytes(u.used)} / ${fmtBytes(u.data_limit)} · ${pct}%</div>
    <div class="bar"><i class="${cls}" style="width:${pct}%"></i></div></div>`;
}

// planControl renders the "apply a plan" select + button for a user row.
function planControl(userId) {
  if (!state.plans.length) return "";
  const opts = state.plans.map((p) => `<option value="${p.id}">${esc(p.name)}</option>`).join("");
  return `<select class="plan-sel" id="sel-${userId}">${opts}</select>
    <button class="btn ghost" onclick="applyPlan('${userId}')" title="Sell / renew this plan">apply</button>`;
}

function renderUsers(users) {
  const tb = document.querySelector("#users tbody");
  document.getElementById("users-empty").style.display = users.length ? "none" : "block";
  tb.innerHTML = users.map((u) => `
    <tr>
      <td>${esc(u.email)}</td>
      <td><span class="badge ${u.enabled ? "on" : "off"}">${u.enabled ? "active" : "disabled"}</span></td>
      <td>${usageCell(u)}</td>
      <td>${fmtDate(u.expires_at)}</td>
      <td class="mono">${esc(u.uuid.slice(0, 8))}…</td>
      <td><div class="row-actions">
        ${planControl(u.id)}
        <button class="btn ghost" onclick="copySub('${u.id}')" title="Copy subscription link">link</button>
        <button class="btn ghost" onclick="resetUser('${u.id}')" title="Reset traffic & re-enable">reset</button>
        <button class="btn ghost danger" onclick="deleteUser('${u.id}','${esc(u.email)}')">del</button>
      </div></td>
    </tr>`).join("");
}

function renderPlans(plans) {
  const tb = document.querySelector("#plans tbody");
  document.getElementById("plans-empty").style.display = plans.length ? "none" : "block";
  tb.innerHTML = plans.map((p) => `
    <tr>
      <td>${esc(p.name)}</td>
      <td>${fmtMoney(p.price_cents, p.currency)}</td>
      <td>${p.duration_days} days</td>
      <td>${p.data_limit ? fmtBytes(p.data_limit) : "∞"}</td>
      <td><div class="row-actions">
        <button class="btn ghost danger" onclick="deletePlan('${p.id}','${esc(p.name)}')">del</button>
      </div></td>
    </tr>`).join("");
}

function renderNodes(nodes) {
  const tb = document.querySelector("#nodes tbody");
  document.getElementById("nodes-empty").style.display = nodes.length ? "none" : "block";
  tb.innerHTML = nodes.map((n) => `
    <tr>
      <td>${esc(n.name)}</td>
      <td class="mono">${esc(n.address)}</td>
      <td><span class="badge ${n.enabled ? "on" : "off"}">${n.enabled ? "enabled" : "disabled"}</span></td>
      <td><div class="row-actions">
        <button class="btn ghost danger" onclick="deleteNode('${n.id}','${esc(n.name)}')">del</button>
      </div></td>
    </tr>`).join("");
}

function renderStats(users, nodes, orders) {
  const active = users.filter((u) => u.enabled).length;
  const paid = orders.filter((o) => o.status === "paid");
  const cur = paid[0] ? paid[0].currency : "USD";
  const revenue = paid.reduce((s, o) => s + (o.amount_cents || 0), 0);
  const cards = [
    { n: users.length, l: "Users" },
    { n: active, l: "Active" },
    { n: nodes.length, l: "Nodes" },
    { n: fmtMoney(revenue, cur), l: "Revenue" },
  ];
  document.getElementById("stats").innerHTML = cards
    .map((c) => `<div class="stat"><div class="n">${c.n}</div><div class="l">${c.l}</div></div>`)
    .join("");
}

// --- actions --------------------------------------------------------------

async function load() {
  try {
    const [users, nodes, plans, orders] = await Promise.all([
      api("GET", "/api/users"), api("GET", "/api/nodes"),
      api("GET", "/api/plans"), api("GET", "/api/orders"),
    ]);
    state.plans = plans || [];
    state.orders = orders || [];
    renderPlans(state.plans);
    renderUsers(users || []);
    renderNodes(nodes || []);
    renderStats(users || [], nodes || [], state.orders);
  } catch (e) {
    if (e.message !== "auth") toast("Load failed: " + e.message, true);
  }
}

async function createUser(ev) {
  ev.preventDefault();
  const f = ev.target;
  const body = { email: f.email.value.trim() };
  const gb = parseFloat(f.limit_gb.value);
  if (gb > 0) body.data_limit = Math.round(gb * 1024 ** 3);
  if (f.expires.value) body.expires_at = new Date(f.expires.value + "T23:59:59Z").toISOString();
  try {
    await api("POST", "/api/users", body);
    f.reset(); toggle("user-form");
    toast("User created");
    load();
  } catch (e) { toast(e.message, true); }
}

async function createPlan(ev) {
  ev.preventDefault();
  const f = ev.target;
  const body = {
    name: f.name.value.trim(),
    price_cents: Math.round(parseFloat(f.price.value) * 100),
    currency: (f.currency.value.trim() || "USD").toUpperCase(),
    duration_days: parseInt(f.days.value, 10),
  };
  const gb = parseFloat(f.limit_gb.value);
  if (gb > 0) body.data_limit = Math.round(gb * 1024 ** 3);
  try {
    await api("POST", "/api/plans", body);
    f.reset(); f.currency.value = "USD"; toggle("plan-form");
    toast("Plan created");
    load();
  } catch (e) { toast(e.message, true); }
}

// applyPlan creates an order for the chosen plan and immediately settles it,
// which extends the user's expiry and quota via the billing logic.
async function applyPlan(userId) {
  const planId = document.getElementById("sel-" + userId).value;
  try {
    const order = await api("POST", "/api/orders", { user_id: userId, plan_id: planId });
    await api("POST", "/api/orders/" + order.id + "/pay");
    toast("Plan applied & paid");
    load();
  } catch (e) { toast(e.message, true); }
}

async function deleteUser(id, email) {
  if (!confirm("Delete user " + email + "?")) return;
  try { await api("DELETE", "/api/users/" + id); toast("User deleted"); load(); }
  catch (e) { toast(e.message, true); }
}

async function resetUser(id) {
  try { await api("POST", "/api/users/" + id + "/reset"); toast("Traffic reset & re-enabled"); load(); }
  catch (e) { toast(e.message, true); }
}

async function deletePlan(id, name) {
  if (!confirm("Delete plan " + name + "?")) return;
  try { await api("DELETE", "/api/plans/" + id); toast("Plan deleted"); load(); }
  catch (e) { toast(e.message, true); }
}

async function deleteNode(id, name) {
  if (!confirm("Remove node " + name + "?")) return;
  try { await api("DELETE", "/api/nodes/" + id); toast("Node removed"); load(); }
  catch (e) { toast(e.message, true); }
}

// boot: apply branding (public), then load data and auto-refresh.
async function boot() {
  try { applyBranding(await api("GET", "/api/branding")); } catch (_) {}
  load();
  setInterval(load, 5000);
}
boot();
