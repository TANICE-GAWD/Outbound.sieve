package main

import (
	"encoding/csv"
	"strconv"
	"strings"
)


type candidate struct {
	Name      string
	Website   string
	Employees int
}


func parseAccountsCSV(raw string) ([]candidate, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}

	nameCol, webCol, empCol := -1, -1, -1
	for i, h := range rows[0] {
		switch h := strings.ToLower(strings.TrimSpace(h)); {
		case nameCol < 0 && (h == "company" || h == "name" || h == "account" || h == "organization"):
			nameCol = i
		case webCol < 0 && (h == "website" || h == "domain" || h == "url" || h == "company domain" || h == "primary domain"):
			webCol = i
		case empCol < 0 && (strings.Contains(h, "employee") || h == "size" || h == "headcount" || h == "# employees"):
			empCol = i
		}
	}
	if nameCol < 0 || webCol < 0 {
		return nil, nil 
	}

	var out []candidate
	for _, row := range rows[1:] {
		if nameCol >= len(row) || webCol >= len(row) {
			continue
		}
		name := strings.TrimSpace(row[nameCol])
		web := strings.TrimSpace(row[webCol])
		if name == "" || web == "" {
			continue
		}
		out = append(out, candidate{Name: name, Website: web, Employees: parseEmployees(row, empCol)})
	}
	return out, nil
}


func parseEmployees(row []string, col int) int {
	if col < 0 || col >= len(row) {
		return -1
	}
	
	cell := strings.NewReplacer(",", "", " ", "").Replace(row[col])
	var digits strings.Builder
	for _, c := range cell {
		if c >= '0' && c <= '9' {
			digits.WriteRune(c)
		} else if digits.Len() > 0 {
			break
		}
	}
	if digits.Len() == 0 {
		return -1
	}
	n, err := strconv.Atoi(digits.String())
	if err != nil {
		return -1
	}
	return n
}


func filterBySize(cands []candidate, max int) (kept []candidate, dropped int) {
	if max <= 0 {
		return cands, 0
	}
	for _, c := range cands {
		if c.Employees > max {
			dropped++
			continue
		}
		kept = append(kept, c)
	}
	return kept, dropped
}
