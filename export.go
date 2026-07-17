package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func packageEngine(jobID string, p Profile, targets []Target) (string, error) {
	if err := os.MkdirAll("out", 0o755); err != nil {
		return "", err
	}
	path := filepath.Join("out", jobID+".zip")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	z := zip.NewWriter(f)
	defer z.Close()

	add := func(name, content string) error {
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}

	rows := make([]map[string]any, len(targets))
	for i, t := range targets {
		rows[i] = t.clayRow()
	}
	clayJSON, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", err
	}

	profileJSON, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}

	files := map[string]string{
		"README.md":           readme(p, targets),
		"icp.json":            string(profileJSON),
		"clay/rows.json":      string(clayJSON),
		"emails/sequence.md":  sequenceMD(targets),
		"emails/linkedin.md":  linkedinMD(targets),
		"emails/followups.md": followupsMD(targets),
		"crm/hubspot.csv":     hubspotCSV(targets),
		"n8n/workflow.json":   n8nWorkflow(p),
	}
	for name, content := range files {
		if err := add(name, content); err != nil {
			return "", err
		}
	}
	return path, nil
}

func readme(p Profile, targets []Target) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GTM Engine — %s\n\n%s\n\n", p.Name, p.Summary)
	fmt.Fprintf(&b, "## ICP\n\n%s\n\n", p.ICP.Description)
	fmt.Fprintf(&b, "- Industries: %s\n", strings.Join(p.ICP.Industries, ", "))
	fmt.Fprintf(&b, "- Size: %s\n", p.ICP.EmployeeRange)
	fmt.Fprintf(&b, "- Geos: %s\n\n", strings.Join(p.ICP.Geos, ", "))
	fmt.Fprintf(&b, "## Personas\n\n")
	for _, persona := range p.Personas {
		fmt.Fprintf(&b, "### %s (%s)\n\n", persona.Title, persona.Seniority)
		fmt.Fprintf(&b, "- Goals: %s\n", strings.Join(persona.Goals, "; "))
		fmt.Fprintf(&b, "- Pains: %s\n", strings.Join(persona.Pains, "; "))
		fmt.Fprintf(&b, "- Triggers: %s\n\n", strings.Join(persona.Triggers, "; "))
	}
	fmt.Fprintf(&b, "## Accounts (%d)\n\n", len(targets))
	fmt.Fprintf(&b, "Firmographics (employee count, annual revenue) are intentionally\n")
	fmt.Fprintf(&b, "blank here — Clay's waterfall fills them. Every website below was fetched\n")
	fmt.Fprintf(&b, "and verified live; descriptions are from the real site, not generated.\n\n")
	fmt.Fprintf(&b, "| Company | Website | ICP Score | Mail | Why kept |\n|---|---|---|---|---|\n")
	for _, t := range targets {
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s |\n", t.Name, t.Website, t.ICPScore, t.MailStatus, t.QualifyReason)
	}
	return b.String()
}

func sequenceMD(targets []Target) string {
	var b strings.Builder
	b.WriteString("# Cold Email Sequence\n\n")
	for _, t := range targets {
		fmt.Fprintf(&b, "## %s (ICP %d)\n\n%s\n\n---\n\n", t.Name, t.ICPScore, t.ColdEmail)
	}
	return b.String()
}

func linkedinMD(targets []Target) string {
	var b strings.Builder
	b.WriteString("# LinkedIn Sequence\n\n")
	for _, t := range targets {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n---\n\n", t.Name, t.LinkedIn)
	}
	return b.String()
}

func followupsMD(targets []Target) string {
	var b strings.Builder
	b.WriteString("# Follow-ups\n\n")
	for _, t := range targets {
		fmt.Fprintf(&b, "## %s\n\n**Follow-up 1**\n\n%s\n\n**Follow-up 2**\n\n%s\n\n---\n\n",
			t.Name, t.Followup1, t.Followup2)
	}
	return b.String()
}

func hubspotCSV(targets []Target) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Write([]string{"Name", "Domain", "Industry", "Description", "ICP Score", "Pain Points", "Mail Status", "Qualify Reason", "Verified At", "Campaign Status"})
	for _, t := range targets {
		w.Write([]string{t.Name, t.Website, t.Industry, t.Summary, strconv.Itoa(t.ICPScore), t.PainPoints, t.MailStatus, t.QualifyReason, t.VerifiedAt, "Ready"})
	}
	w.Flush()
	return b.String()
}

func n8nWorkflow(p Profile) string {
	wf := map[string]any{
		"name": p.Name + " — GTM Engine",
		"nodes": []map[string]any{
			{
				"parameters":  map[string]any{"path": "gtm-engine", "options": map[string]any{}},
				"name":        "Clay Webhook",
				"type":        "n8n-nodes-base.webhook",
				"typeVersion": 1,
				"position":    []int{240, 300},
			},
			{
				"parameters": map[string]any{
					"method":      "POST",
					"url":         groqURL,
					"sendHeaders": true,
					"sendBody":    true,
				},
				"name":        "Groq — Personalize",
				"type":        "n8n-nodes-base.httpRequest",
				"typeVersion": 4,
				"position":    []int{460, 300},
			},
			{
				"parameters":  map[string]any{"resource": "company", "operation": "create"},
				"name":        "HubSpot",
				"type":        "n8n-nodes-base.hubspot",
				"typeVersion": 2,
				"position":    []int{680, 300},
			},
			{
				"parameters":  map[string]any{"select": "channel", "text": "New GTM-qualified account"},
				"name":        "Slack",
				"type":        "n8n-nodes-base.slack",
				"typeVersion": 2,
				"position":    []int{900, 300},
			},
		},
		"connections": map[string]any{
			"Clay Webhook":       map[string]any{"main": [][]map[string]any{{{"node": "Groq — Personalize", "type": "main", "index": 0}}}},
			"Groq — Personalize": map[string]any{"main": [][]map[string]any{{{"node": "HubSpot", "type": "main", "index": 0}}}},
			"HubSpot":            map[string]any{"main": [][]map[string]any{{{"node": "Slack", "type": "main", "index": 0}}}},
		},
	}
	out, _ := json.MarshalIndent(wf, "", "  ")
	return string(out)
}
