package main

// ICP is what Groq infers from the user's own site.
type ICP struct {
	Description   string   `json:"description"`
	Industries    []string `json:"industries"`
	EmployeeRange string   `json:"employee_range"`
	Geos          []string `json:"geos"`
	Keywords      []string `json:"keywords"`
	PainPoints    []string `json:"pain_points"`
}

type Persona struct {
	Title     string   `json:"title"`
	Seniority string   `json:"seniority"`
	Goals     []string `json:"goals"`
	Pains     []string `json:"pains"`
	Triggers  []string `json:"triggers"`
}

// Profile describes the user's own company.
type Profile struct {
	Name      string    `json:"name"`
	Website   string    `json:"website"`
	Summary   string    `json:"summary"`
	Industry  string    `json:"industry"`
	ValueProp string    `json:"value_prop"`
	ICP       ICP       `json:"icp"`
	Personas  []Persona `json:"personas"`
}

// Target is a candidate account. Name/Website come from Groq; Description is
// crawled from the live site. Firmographics (employees, revenue, funding, tech
// stack) are deliberately absent — Clay's waterfall fills those, and we do not
// invent them.
type Target struct {
	Name        string `json:"name"`
	Website     string `json:"website"`
	Description string `json:"description"` // crawled, real
	Industry    string `json:"industry"`
	ICPScore    int    `json:"icp_score"`
	Summary     string `json:"summary"`
	PainPoints  string `json:"pain_points"`
	ValueProp   string `json:"value_prop"`
	OpeningLine string `json:"opening_line"`
	ColdEmail   string `json:"cold_email"`
	LinkedIn    string `json:"linkedin_message"`
	Followup1   string `json:"followup_1"`
	Followup2   string `json:"followup_2"`
}

// clayRow maps a Target to the Companies table payload.
//
// ponytail: field names must match Clay column names character for character,
// capitalization included, or Clay drops them silently. Keep this map and the
// workspace columns in sync by hand — there is no API to verify against.
func (t Target) clayRow() map[string]any {
	return map[string]any{
		"Company":         t.Name,
		"Website":         t.Website,
		"Industry":        t.Industry,
		"Company Summary": t.Summary,
		"ICP Score":       t.ICPScore,
		"Pain Points":     t.PainPoints,
		"Value Prop":      t.ValueProp,
		"Opening Line":    t.OpeningLine,
		"Cold Email":      t.ColdEmail,
		"LinkedIn":        t.LinkedIn,
		"Campaign Status": "Ready",
	}
}
