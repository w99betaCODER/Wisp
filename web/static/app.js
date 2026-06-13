"use strict";

// --- tiny helpers ---------------------------------------------------------

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  if (!res.ok) {
    let msg = res.status + " " + res.statusText;
    try { const j = await res.json(); if (j.error) msg = j.error; } catch (_) {}
    throw new Error(msg);
  }
  return res.status === 204 ? null : res.json();
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

function fmtDate(s) {
  if (!s) return "never";
  const d = new Date(s);
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
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
        <button class="btn ghost" onclick="copySub('${u.id}')" title="Copy subscription link">link</button>
        <button class="btn ghost" onclick="resetUser('${u.id}')" title="Reset traffic & re-enable">reset</button>
        <button class="btn ghost danger" onclick="deleteUser('${u.id}','${esc(u.email)}')">del</button>
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

function renderStats(users, nodes) {
  const active = users.filter((u) => u.enabled).length;
  const traffic = users.reduce((s, u) => s + (u.used || 0), 0);
  const cards = [
    { n: users.length, l: "Users" },
    { n: active, l: "Active" },
    { n: nodes.length, l: "Nodes" },
    { n: fmtBytes(traffic), l: "Traffic used" },
  ];
  document.getElementById("stats").innerHTML = cards
    .map((c) => `<div class="stat"><div class="n">${c.n}</div><div class="l">${c.l}</div></div>`)
    .join("");
}

// --- actions --------------------------------------------------------------

async function load() {
  try {
    const [users, nodes] = await Promise.all([api("GET", "/api/users"), api("GET", "/api/nodes")]);
    renderUsers(users || []);
    renderNodes(nodes || []);
    renderStats(users || [], nodes || []);
  } catch (e) {
    toast("Load failed: " + e.message, true);
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

async function createNode(ev) {
  ev.preventDefault();
  const f = ev.target;
  try {
    await api("POST", "/api/nodes", { name: f.name.value.trim(), address: f.address.value.trim() });
    f.reset(); toggle("node-form");
    toast("Node registered");
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

async function deleteNode(id, name) {
  if (!confirm("Remove node " + name + "?")) return;
  try { await api("DELETE", "/api/nodes/" + id); toast("Node removed"); load(); }
  catch (e) { toast(e.message, true); }
}

// initial load + light auto-refresh
load();
setInterval(load, 5000);
