package main

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Outbound.sieve</title>
<style>
  :root { color-scheme: dark; --bg:#0a0a0b; --fg:#ededef; --dim:#8b8b93;
          --line:#232327; --card:#111113; --ok:#4ade80; --err:#f87171; --run:#60a5fa; }
  * { box-sizing:border-box; }
  body { margin:0; background:var(--bg); color:var(--fg); font:15px/1.6 ui-sans-serif,system-ui,-apple-system,sans-serif; }
  main { max-width:820px; margin:0 auto; padding:64px 24px 96px; }
  h1 { font-size:28px; letter-spacing:-.02em; margin:0 0 6px; }
  .sub { color:var(--dim); margin:0 0 32px; }
  form { display:flex; gap:8px; flex-wrap:wrap; }
  input { flex:1; min-width:240px; background:var(--card); border:1px solid var(--line);
          color:var(--fg); padding:11px 14px; border-radius:8px; font-size:15px; }
  input:focus { outline:none; border-color:#3f3f46; }
  button { background:var(--fg); color:#000; border:0; padding:11px 20px; border-radius:8px;
           font-weight:600; font-size:15px; cursor:pointer; }
  button:disabled { opacity:.4; cursor:default; }
  details { margin-top:12px; color:var(--dim); font-size:13px; }
  details input { margin-top:8px; width:100%; }
  textarea { margin-top:8px; width:100%; background:var(--card); border:1px solid var(--line);
             color:var(--fg); padding:11px 14px; border-radius:8px; font:13px/1.5 ui-monospace,monospace; resize:vertical; }
  textarea:focus { outline:none; border-color:#3f3f46; }
  .steps { margin-top:36px; display:flex; flex-direction:column; gap:2px; }
  .step { display:flex; gap:10px; align-items:baseline; padding:7px 0; font-variant-numeric:tabular-nums; }
  .step .icon { width:16px; flex:none; }
  .step .detail { color:var(--dim); font-size:13px; }
  .running .icon { color:var(--run); }
  .done .icon { color:var(--ok); }
  .error .icon, .error .detail { color:var(--err); }
  .skipped .icon, .skipped .detail { color:var(--dim); }
  table { width:100%; border-collapse:collapse; margin-top:32px; font-size:14px; }
  th,td { text-align:left; padding:9px 10px; border-bottom:1px solid var(--line); vertical-align:top; }
  th { color:var(--dim); font-weight:500; font-size:12px; text-transform:uppercase; letter-spacing:.04em; }
  .score { font-variant-numeric:tabular-nums; text-align:right; }
  .mail { text-transform:capitalize; font-size:13px; }
  .mail.deliverable { color:var(--ok); }
  .mail.risky { color:#fbbf24; }
  .mail.invalid { color:var(--err); }
  .wrap { overflow-x:auto; }
  a.dl { display:inline-block; margin-top:28px; background:var(--ok); color:#000;
         padding:11px 20px; border-radius:8px; font-weight:600; text-decoration:none; }
  .note { color:var(--dim); font-size:13px; margin-top:10px; }
</style>
</head>
<body>
<main>
  <h1>Outbound.sieve</h1>
  <p class="sub">Enter a website. Get a populated Clay workspace.</p>

  <form id="f">
    <input id="site" placeholder="allmind.ai" autocomplete="off" required>
    <button id="go">Build engine</button>
    <details>
      <summary>Clay webhook (optional)</summary>
      <input id="hook" placeholder="https://api.clay.com/v3/sources/webhook/..." autocomplete="off">
      <input id="tok" placeholder="auth token (optional)" autocomplete="off">
    </details>
    <details>
      <summary>Account list — Apollo CSV (optional)</summary>
      <p class="note">Paste an Apollo export with Company and Website/Domain columns. When set, these become the accounts instead of the model guessing them, and the size cap is a hard filter.</p>
      <textarea id="csv" rows="4" placeholder="Company,Website,# Employees&#10;Acme,acme.com,140" autocomplete="off"></textarea>
      <input id="maxemp" type="number" min="0" placeholder="max employees (e.g. 500) — blank = no cap" autocomplete="off">
    </details>
  </form>

  <div class="steps" id="steps"></div>
  <div id="out"></div>
</main>
<script>
const $ = s => document.querySelector(s);
const icons = { running:"◌", done:"✓", error:"✕", skipped:"–" };
let els = {};

$("#f").onsubmit = async e => {
  e.preventDefault();
  $("#go").disabled = true;
  $("#steps").innerHTML = ""; $("#out").innerHTML = ""; els = {};

  const r = await fetch("/api/jobs", {
    method:"POST", headers:{"Content-Type":"application/json"},
    body: JSON.stringify({ website:$("#site").value, clay_webhook:$("#hook").value, clay_token:$("#tok").value, accounts_csv:$("#csv").value, max_employees:parseInt($("#maxemp").value)||0 })
  });
  if (!r.ok) { $("#out").textContent = await r.text(); $("#go").disabled = false; return; }
  const { id } = await r.json();

  const es = new EventSource("/api/jobs/" + id + "/events");
  es.onmessage = m => {
    const ev = JSON.parse(m.data);
    let el = els[ev.step];
    if (!el) {
      el = document.createElement("div");
      el.innerHTML = '<span class="icon"></span><span class="label"></span><span class="detail"></span>';
      el.querySelector(".label").textContent = ev.step;
      $("#steps").append(el); els[ev.step] = el;
    }
    el.className = "step " + ev.status;
    el.querySelector(".icon").textContent = icons[ev.status] || "◌";
    el.querySelector(".detail").textContent = ev.detail || "";
  };
  es.addEventListener("end", async () => { es.close(); $("#go").disabled = false; await show(id); });
  es.onerror = () => { es.close(); $("#go").disabled = false; };
};

async function show(id) {
  const d = await (await fetch("/api/jobs/" + id + "/result")).json();
  if (!d.targets || !d.targets.length) return;
  const rows = d.targets.map(t => "<tr><td>" + esc(t.name) + "</td><td>" + esc(t.website) +
    "</td><td class='score'>" + t.icp_score + "</td><td class='mail " + esc(t.mail_status || "") + "'>" +
    esc(t.mail_status || "") + "</td><td class='detail'>" + esc(t.qualify_reason || "") + "</td></tr>").join("");
  $("#out").innerHTML =
    "<div class='wrap'><table><thead><tr><th>Company</th><th>Website</th>" +
    "<th class='score'>ICP</th><th>Mail</th><th>Why kept</th></tr></thead><tbody>" + rows + "</tbody></table></div>" +
    (d.download ? "<a class='dl' href='/api/jobs/" + id + "/download'>Download GTM engine</a>" : "") +
    "<p class='note'>Employee count and annual revenue are intentionally blank. " +
    "Clay's waterfall fills those.</p>";
}
const esc = s => String(s).replace(/[&<>"']/g, c =>
  ({ "&":"&amp;", "<":"&lt;", ">":"&gt;", '"':"&quot;", "'":"&#39;" }[c]));
</script>
</body>
</html>`
