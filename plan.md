# Outbound.sieve

A GTM engine builder. Enter a website, get a populated Clay workspace.

Built for a Clay-heavy role, so **the Clay workspace is the deliverable**. The Go
app is what proves you can feed it at volume.

---

## The correction that shapes everything

Clay has no public API for creating workspaces, tables, columns, or views.
The HTTP/webhook integration pushes **rows into a table that already exists**.
Creating structure via API is an open feature request, not a shipped capability.

So the workspace is **hand-built once, by hand, as a template**. That is not a
workaround — it is the thing being evaluated. A GTM engineer's craft lives in the
workspace, not in a backend.

```
You build the Clay template once (UI work, by hand)
        ↓
User duplicates the workbook, pastes the webhook URL into the app
        ↓
Go app crawls → researches → scores → POSTs rows in
        ↓
Clay's waterfalls, run conditions, lookups, and views fire on arrival
        ↓
"Here's your GTM engine." Open Clay. Fully populated.
```

---

## Credits are COGS, and that's the pitch

Clay free tier: **100 Data Credits/month, 500 Actions, 200 rows/table.** Claygent
and enrichment columns bill per cell. Seven AI columns × seven enrichment columns
per row would exhaust the month on one demo run.

So the AI runs in Groq, inside the Go pipeline, and arrives in Clay as populated
text. **Say this out loud in the demo** — it's an architecture call under cost
pressure, which is exactly what an agency needs from a GTM hire:

> "Claygent browses, so I use it where browsing is the point. Email copy is just
> text generation — that's Groq, at zero marginal cost. Here's where I drew the
> line."

The free tier isn't an apology. It's the evidence.

| Job | Runs in | Why |
|---|---|---|
| Company summary, pain points, value prop | Groq | Pure text gen, no browsing |
| Opening lines, cold email, LinkedIn, follow-ups | Groq | Same, and it's the bulk |
| Waterfall enrichment | Clay | Only exists in Clay |
| Run conditions, lookups, formulas, views | Clay | Only exists in Clay |
| Claygent | Clay, a few rows only | Proves capability, survives the budget |

---

## The Clay workspace

The submission. Spend the 100 credits on what only Clay does.

### Companies table

```
Company · Website · Industry · Employees · Revenue · Funding · Tech Stack
ICP Score · Pain Points · Company Summary · Value Prop
```

### People table

```
First Name · Last Name · Role · LinkedIn · Email · Location
Decision Maker Score · Opening Line · Cold Email · Campaign Status
```

### The four things a GTM lead scans for

1. **A waterfall** — provider 1 → fall back to 2 → fall back to 3, stop when
   found. Two deep on a handful of rows is enough to prove you know why
   waterfalls exist.
2. **Run conditions** — enrichment fires only when ICP Score clears a threshold.
   Credit discipline made visible in the workspace itself.
3. **A lookup** — people table joined to companies table. Fiddly in Clay, obvious
   to anyone who's done it.
4. **Formulas + views + dedup** — cheap, and it's what makes it an engine instead
   of a spreadsheet.

### Views

```
All Companies · High ICP · Ready For Outreach · Needs Enrichment · Completed
```

---

## Contacts: no Apollo

Apollo API access requires the Organization plan, $119/user/month, 3-user
minimum — ~$357/month to make one call. Free tier is 100 email credits and
explicitly no API.

- **v1: company-only.** Crawl → Groq → ICP, personas, scoring → companies table.
  Works end to end on zero paid keys.
- **v2: manual CSV drop.** Export from Apollo's free UI by hand, drop the file in,
  the app enriches and pushes to the people table. Real contacts, no API bill,
  one click of human work in a demo you're driving anyway.

**No fabricated leads.** Groq-invented names and emails that read as real
contacts don't go in the people table. If placeholder rows are needed for the
visual, they're labeled sample data.

---

## Backend

One Go binary, **zero dependencies**. No queue, no database, no object storage,
no framework. Flat package, one file per job.

```
main.go       server, SSE, in-memory job registry
pipeline.go   the 10 steps, in order, + prompts
crawl.go      http.Get + meta tags + strip tags
groq.go       Groq client, JSON mode, rate-limit handling
score.go      deterministic ICP scoring
clay.go       POST rows to webhook
export.go     zip on disk
ui.go         the embedded page
sieve_test.go
```

Gin is gone: Go 1.22+ routes with `http.HandleFunc("POST /api/jobs", ...)` and
SSE is `http.Flusher`. A router dependency would earn nothing here.

### Cut, and when to add it back

| Cut | Replaced by | Add when |
|---|---|---|
| Redis queue + worker pool | goroutine + channel | more than one machine |
| PostgreSQL + Neon | `map[string]*Job` + mutex | job history outlives the process |
| Cloudflare R2 | zip on disk | files must cross machines |
| Firecrawl | `http.Get` + strip tags | a JS-heavy site returns empty text |
| CompanyEnrich | Apollo already returns firmographics | Apollo's company data proves thin |
| summary.pdf | the Clay workspace is the report | never |
| TanStack Query | `EventSource` | — |
| Framer Motion | CSS | — |

Known ceilings, marked in code as `ponytail:` comments:

- Clay webhook sources cap at **50,000 submissions**, and the count survives row
  deletion. A long-lived demo template eventually needs a fresh webhook.
- Webhook payload field names must match Clay column names **character for
  character, capitalization included**. Silent failure otherwise.
- **Groq free tier is 12k tokens/minute.** Copy calls are trimmed to 3k chars of
  site text and run 2-at-a-time to fit; the client honors Groq's `retry-after`.
  Raising `targetCount` past ~10 will start stretching runs across TPM windows.
- **Client-rendered sites** (Lovable, Framer, most SPA builders) ship an empty
  body. The crawler reads `<head>` — title, meta description, OG tags, JSON-LD —
  which is the only real copy on such a page. loopgtm.ai is one of these: 45
  chars of body text, and its ICP exists solely in the meta description.
- **Account discovery is LLM recall**, so it skews toward household names. Every
  domain is fetched and unreachable ones are dropped, which kills invented
  companies but cannot enforce company *size*. A real size filter needs Apollo
  or a company database. This is the strongest argument for the CSV drop.

---

## Pipeline

The SSE progress feed. Labels pushed onto a channel — the Cursor feel costs
nothing.

```
✓ Crawling Website
✓ Company Research
✓ ICP Detection
✓ Buyer Personas
✓ Lead Scoring
✓ Writing Opening Lines
✓ Writing Email Sequence
✓ Writing LinkedIn Sequence
✓ Pushing to Clay Workspace
✓ Packaging GTM Engine
```

---

## Frontend

Next.js + Tailwind + shadcn. One page.

```
Enter Website → Live Pipeline (SSE) → Workspace Preview → Download
```

---

## Export

```
outbound-sieve/
    clay/           rows.json (what got pushed)
    emails/         sequence.md · linkedin.md · followups.md
    crm/            hubspot.csv · contacts.csv
    n8n/            workflow.json    (static template, IDs substituted)
```

---

## Stack

**Frontend** Next.js · React · Tailwind · shadcn/ui
**Backend** Go · Gin
**AI** Groq
**Platform** Clay
**Deploy** Docker · Fly.io

Keys needed: **Groq.** That's it.

---

## Ship

Send Bolaji three things:

1. **The Clay template share link** — the actual deliverable.
2. **The repo.**
3. **A short Loom** — walk the workspace (waterfall, run condition, lookup,
   views), then the app filling it live from a URL.

That's "I built the engine and the thing that loads it," not "I made a nice
table."
