package main

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

type Profile struct {
	Name      string    `json:"name"`
	Website   string    `json:"website"`
	Summary   string    `json:"summary"`
	Industry  string    `json:"industry"`
	ValueProp string    `json:"value_prop"`
	ICP       ICP       `json:"icp"`
	Personas  []Persona `json:"personas"`
}

type Target struct {
	Name        string `json:"name"`
	Website     string `json:"website"`
	Description string `json:"description"`
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

	// Provenance — every keep/drop decision explains itself.
	Qualified     bool   `json:"qualified"`
	QualifyReason string `json:"qualify_reason"`
	MailStatus    string `json:"mail_status"`
	MailReason    string `json:"mail_reason"`
	VerifiedAt    string `json:"verified_at"`
}

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
		"Qualify Reason":  t.QualifyReason,
		"Mail Status":     t.MailStatus,
		"Verified At":     t.VerifiedAt,
		"Campaign Status": "Ready",
	}
}
