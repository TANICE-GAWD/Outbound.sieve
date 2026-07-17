package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const targetCount = 10

func runPipeline(ctx context.Context, job *Job) {
	defer job.finish()

	job.start("Crawling Website")
	site := fetchSite(ctx, job.Website)
	if site == "" {
		job.fail("Crawling Website", "couldn't read that site — is the URL right, and does it serve HTML?")
		return
	}
	if len(site) < thinSite {
		job.ok("Crawling Website", fmt.Sprintf("%d chars — client-rendered, running on meta tags", len(site)))
	} else {
		job.ok("Crawling Website", fmt.Sprintf("%d chars", len(site)))
	}

	job.start("Company Research")
	var p Profile
	err := groqJSON(ctx, sysAnalyst, fmt.Sprintf(`Website: %s

Site content:
%s

Return JSON: {"name","summary","industry","value_prop"} — summary is 2 sentences, grounded only in the content above.`, job.Website, site), &p)
	if err != nil {
		job.fail("Company Research", err.Error())
		return
	}
	p.Website = normalizeURL(job.Website)
	job.ok("Company Research", p.Name)

	job.start("ICP Detection")
	var icpWrap struct {
		ICP ICP `json:"icp"`
	}
	err = groqJSON(ctx, sysAnalyst, fmt.Sprintf(`Company: %s — %s
Value prop: %s

Site content:
%s

Infer the ideal customer profile. Return JSON:
{"icp":{"description","industries":[],"employee_range","geos":[],"keywords":[],"pain_points":[]}}
keywords: 6-10 lowercase terms that would literally appear on a good-fit company's website.
pain_points: 3-5 lowercase phrases describing what that company struggles with.`,
		p.Name, p.Summary, p.ValueProp, site), &icpWrap)
	if err != nil {
		job.fail("ICP Detection", err.Error())
		return
	}
	p.ICP = icpWrap.ICP
	job.ok("ICP Detection", strings.Join(p.ICP.Industries, ", "))

	job.start("Buyer Personas")
	var personaWrap struct {
		Personas []Persona `json:"personas"`
	}
	err = groqJSON(ctx, sysAnalyst, fmt.Sprintf(`ICP: %s
Industries: %s
Selling: %s

Return JSON: {"personas":[{"title","seniority","goals":[],"pains":[],"triggers":[]}]}
Exactly 3 personas in the buying committee.`,
		p.ICP.Description, strings.Join(p.ICP.Industries, ", "), p.ValueProp), &personaWrap)
	if err != nil {
		job.fail("Buyer Personas", err.Error())
		return
	}
	p.Personas = personaWrap.Personas
	titles := make([]string, len(p.Personas))
	for i, persona := range p.Personas {
		titles[i] = persona.Title
	}
	job.ok("Buyer Personas", strings.Join(titles, ", "))
	job.setProfile(p)

	job.start("Finding Target Accounts")
	var cands []candidate
	if strings.TrimSpace(job.AccountsCSV) != "" {
		// CSV path: real companies with real headcounts. Size filtering happens in
		// the shared step below so both paths get the same hard cap.
		parsed, perr := parseAccountsCSV(job.AccountsCSV)
		if perr != nil {
			job.fail("Finding Target Accounts", "couldn't parse that CSV: "+perr.Error())
			return
		}
		if len(parsed) == 0 {
			job.fail("Finding Target Accounts", "CSV had no usable rows — need a Company column and a Website/Domain column")
			return
		}
		cands = parsed
		job.ok("Finding Target Accounts", fmt.Sprintf("%d from CSV", len(cands)))
	} else {
		var candWrap struct {
			Companies []struct {
				Name     string `json:"name"`
				Website  string `json:"website"`
				SizeTier string `json:"size_tier"`
			} `json:"companies"`
		}
		err = groqJSON(ctx, sysProspector, fmt.Sprintf(`ICP: %s
Industries: %s
Size: %s
Geos: %s

Name %d REAL companies that fit this ICP, with their real primary domain.

Who to name — this IS the task, not a hint:
- Buyable firms only. The buyer must be able to say yes fast, without a
  procurement committee or a year-long enterprise cycle: boutique, emerging, and
  lower-mid-market, where one champion (a founder, a partner, a team lead) has
  the authority to sign.
- Firms that FEEL the pain — small enough that expensive incumbent tools hurt,
  new enough to try something better.

Who to EXCLUDE, hard:
- Any globally-recognized or household-name institution. If a professional in
  this field would instantly know the name, or it's one of the largest players
  in the category, or you'd read about it in the business press — leave it out.
  It will not buy from a small vendor in a cycle that matters, and naming it just
  signals you defaulted to the obvious.
- %s itself, and its direct competitors.

Before each pick, ask: "could a small, early-stage vendor realistically close
this account this quarter?" If not, drop it and name a smaller peer instead.

For each firm also return size_tier — its HONEST size:
  "boutique" (small / emerging), "mid" (lower-mid-market), or
  "large" (a major, established institution).
Be accurate. Large firms are removed automatically, so if a firm is genuinely
large the right move is to not name it at all — never relabel it to sneak it in.
Return JSON: {"companies":[{"name","website","size_tier"}]}`,
			p.ICP.Description, strings.Join(p.ICP.Industries, ", "), p.ICP.EmployeeRange,
			strings.Join(p.ICP.Geos, ", "), targetCount+14, p.Name), &candWrap)
		if err != nil {
			job.fail("Finding Target Accounts", err.Error())
			return
		}
		var tooBig int
		for _, c := range candWrap.Companies {
			if strings.EqualFold(strings.TrimSpace(c.SizeTier), "large") {
				tooBig++ // model admits it's a major institution — not buyable, drop it
				continue
			}
			// -1, not 0: unknown headcount, so the qualify gate enriches and
			// flags it rather than treating it as a 0-employee company.
			cands = append(cands, candidate{Name: c.Name, Website: c.Website, Employees: -1})
		}
		detail := fmt.Sprintf("%d candidates", len(cands))
		if tooBig > 0 {
			detail = fmt.Sprintf("%d candidates, %d too large dropped", len(cands), tooBig)
		}
		job.ok("Finding Target Accounts", detail)
	}

	// Headcount: enrich unknowns whenever we have a key (so the qualify gate can
	// judge against the ICP's real size band, not just the optional form cap),
	// then apply the hard cap as a cheap pre-fetch drop. Unknown stays -1 and is
	// kept-but-flagged at qualify time, never silently dropped.
	if key := os.Getenv("ABSTRACT_API_KEY"); key != "" || job.MaxEmployees > 0 {
		job.start("Enriching Headcount")
		enriched := 0
		if key != "" {
			var (
				wgE  sync.WaitGroup
				semE = make(chan struct{}, 4) // Abstract free tier is ~1 req/sec; 4 is polite
			)
			for i := range cands {
				if cands[i].Employees >= 0 {
					continue // CSV already gave us a number; don't spend a credit
				}
				wgE.Add(1)
				go func(i int) {
					defer wgE.Done()
					semE <- struct{}{}
					defer func() { <-semE }()
					if n, ok := enrichEmployees(ctx, domainOf(cands[i].Website), key); ok {
						cands[i].Employees = n // distinct index per goroutine, no race
					}
				}(i)
			}
			wgE.Wait()
			for _, c := range cands {
				if c.Employees >= 0 {
					enriched++
				}
			}
		}
		dropped := 0
		if job.MaxEmployees > 0 {
			var kept []candidate
			kept, dropped = filterBySize(cands, job.MaxEmployees)
			cands = kept
			if len(cands) == 0 {
				job.fail("Enriching Headcount", fmt.Sprintf("every candidate was over the %d-employee cap", job.MaxEmployees))
				return
			}
		}
		job.ok("Enriching Headcount", fmt.Sprintf("%d/%d headcounts, %d over cap dropped", enriched, len(cands)+dropped, dropped))
	}

	job.start("Verifying Accounts")
	type verified struct {
		Target
		employees int
		site      string
	}
	var (
		mu   sync.Mutex
		outs []verified
		wg   sync.WaitGroup
		sem  = make(chan struct{}, 6)
	)
	for _, c := range cands {
		if c.Website == "" || c.Name == "" {
			continue
		}
		wg.Add(1)
		go func(name, web string, emp int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			text := fetchSite(ctx, web)
			if text == "" {
				return
			}
			mu.Lock()
			outs = append(outs, verified{
				Target:    Target{Name: name, Website: normalizeURL(web), Description: text},
				employees: emp,
				site:      text,
			})
			mu.Unlock()
		}(c.Name, c.Website, c.Employees)
	}
	wg.Wait()
	if len(outs) == 0 {
		job.fail("Verifying Accounts", "no candidate domain resolved — try a more specific website")
		return
	}
	job.ok("Verifying Accounts", fmt.Sprintf("%d of %d live", len(outs), len(cands)))

	// ICP gate: every keep/drop carries a reason, so the filtering is auditable
	// instead of a black box. Off-ICP accounts are dropped here.
	job.start("Qualifying Accounts")
	{
		kept := outs[:0]
		for _, v := range outs {
			pass, reason := qualify(candidate{Name: v.Name, Website: v.Website, Employees: v.employees}, v.site, p.ICP)
			v.Qualified = pass
			v.QualifyReason = reason
			if pass {
				kept = append(kept, v)
			}
		}
		dropped := len(outs) - len(kept)
		outs = kept
		if len(outs) == 0 {
			job.fail("Qualifying Accounts", "no account matched the ICP size band and industry/keywords")
			return
		}
		job.ok("Qualifying Accounts", fmt.Sprintf("%d qualified, %d off-ICP dropped", len(outs), dropped))
	}

	// Deliverability: a domain with no mail server is a guaranteed bounce. Drop
	// those; keep and flag free/throwaway domains as risky. Domain-level only —
	// not per-mailbox proof.
	job.start("Verifying Deliverability")
	{
		var (
			wgD  sync.WaitGroup
			semD = make(chan struct{}, 8)
		)
		for i := range outs {
			wgD.Add(1)
			go func(i int) {
				defer wgD.Done()
				semD <- struct{}{}
				defer func() { <-semD }()
				st, rz := mailDeliverability(ctx, domainOf(outs[i].Website))
				outs[i].MailStatus = st // distinct index, no race
				outs[i].MailReason = rz
			}(i)
		}
		wgD.Wait()

		kept := outs[:0]
		var dead, deliverable, risky int
		for _, v := range outs {
			switch v.MailStatus {
			case "invalid":
				dead++
				continue
			case "deliverable":
				deliverable++
			case "risky":
				risky++
			}
			kept = append(kept, v)
		}
		outs = kept
		if len(outs) == 0 {
			job.fail("Verifying Deliverability", "every domain was a guaranteed bounce (no mail server)")
			return
		}
		job.ok("Verifying Deliverability", fmt.Sprintf("%d deliverable, %d risky, %d dead dropped", deliverable, risky, dead))
	}

	job.start("Lead Scoring")
	for i := range outs {
		outs[i].ICPScore = scoreICP(outs[i].site, p.ICP)
	}
	sort.Slice(outs, func(i, j int) bool { return outs[i].ICPScore > outs[j].ICPScore })
	if len(outs) > targetCount {
		outs = outs[:targetCount]
	}
	job.ok("Lead Scoring", fmt.Sprintf("top score %d", outs[0].ICPScore))

	job.start("Writing Personalization")
	sem = make(chan struct{}, 2)
	wg = sync.WaitGroup{}
	var copyFails atomic.Int32
	for i := range outs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var c struct {
				Industry    string `json:"industry"`
				Summary     string `json:"summary"`
				PainPoints  string `json:"pain_points"`
				ValueProp   string `json:"value_prop"`
				OpeningLine string `json:"opening_line"`
				ColdEmail   string `json:"cold_email"`
				LinkedIn    string `json:"linkedin_message"`
				Followup1   string `json:"followup_1"`
				Followup2   string `json:"followup_2"`
			}

			site := outs[i].site
			if len(site) > 3000 {
				site = site[:3000]
			}
			err := groqJSON(ctx, sysCopywriter, fmt.Sprintf(`We are %s (%s). We sell: %s

Target account: %s (%s)
Their site says:
%s

Write outbound for this specific account, grounded in their site content.
You are writing to %s, the persona most likely to own this problem.

Return JSON: {"industry","summary","pain_points","value_prop","opening_line",
"cold_email","linkedin_message","followup_1","followup_2"}

opening_line: one sentence quoting or naming something concrete from THEIR site
  (a service, a market, a claim they make). No flattery, no "I noticed".
cold_email: 60-90 words. Must reference that same concrete detail, state one
  specific outcome, and end with one low-friction CTA. Never open with
  "Discover how" / "I hope" / "Let's discuss" — those say nothing.
pain_points: what THEY struggle with, inferred from their site, not from us.

Our only website is %s. Never write any other URL for us, and never invent one.`,
				p.Name, p.Website, p.ValueProp, outs[i].Name, outs[i].Website, site,
				primaryPersona(p), p.Website), &c)
			if err != nil {
				copyFails.Add(1)
				log.Printf("copy failed for %s: %v", outs[i].Name, err)
				return
			}
			outs[i].Industry = c.Industry
			outs[i].Summary = c.Summary
			outs[i].PainPoints = c.PainPoints
			outs[i].ValueProp = c.ValueProp
			outs[i].OpeningLine = c.OpeningLine
			outs[i].ColdEmail = c.ColdEmail
			outs[i].LinkedIn = c.LinkedIn
			outs[i].Followup1 = c.Followup1
			outs[i].Followup2 = c.Followup2
		}(i)
	}
	wg.Wait()

	now := time.Now().UTC().Format(time.RFC3339)
	targets := make([]Target, len(outs))
	for i, v := range outs {
		targets[i] = v.Target
		targets[i].Description = ""
		targets[i].VerifiedAt = now
	}
	job.setTargets(targets)
	if n := copyFails.Load(); n > 0 {
		job.ok("Writing Personalization", fmt.Sprintf("%d accounts, %d without copy (Groq failed)", len(targets), n))
	} else {
		job.ok("Writing Personalization", fmt.Sprintf("%d accounts", len(targets)))
	}

	job.start("Pushing to Clay Workspace")
	rows := make([]map[string]any, len(targets))
	for i, t := range targets {
		rows[i] = t.clayRow()
	}
	if job.ClayWebhook == "" {
		job.skip("Pushing to Clay Workspace", "no webhook set — rows are in the export")
	} else {
		sent, err := pushToClay(ctx, job.ClayWebhook, job.ClayToken, rows)
		if err != nil {
			job.fail("Pushing to Clay Workspace", err.Error())
			return
		}
		job.ok("Pushing to Clay Workspace", fmt.Sprintf("%d rows", sent))
	}

	job.start("Packaging GTM Engine")
	path, err := packageEngine(job.ID, p, targets)
	if err != nil {
		job.fail("Packaging GTM Engine", err.Error())
		return
	}
	job.setZip(path)
	job.ok("Packaging GTM Engine", "ready")
}

func primaryPersona(p Profile) string {
	if len(p.Personas) == 0 {
		return "the decision maker"
	}
	return p.Personas[0].Title
}

const sysAnalyst = `You are a GTM analyst. You are given real website content.
Ground every claim in that content. If something isn't in the content, leave it
out rather than guessing. Reply with JSON only.`

const sysProspector = `You are a GTM researcher naming real companies that match
an ICP. Only name companies you are confident actually exist, with their real
primary domain (e.g. "stripe.com"). Never invent a company or a domain — every
domain you return is fetched and checked, and wrong ones are discarded.

Right-size every pick: name firms a small vendor could actually close, not the
biggest names in the category. A famous logo you can't win is worthless here.
Reply with JSON only.`

const sysCopywriter = `You are a senior outbound copywriter. Write specific,
grounded, human copy. No flattery openers, no "I hope this finds you well", no
em dashes. Reference only what the target's site actually says.

Never invent numbers. No statistics, percentages, timeframes, customer counts or
results unless they were given to you above. "reduce time-to-hire by 30%" is a
fabrication if no one said 30% — write the claim without the number instead.
A vague true sentence beats a specific invented one.

Reply with JSON only.`
