package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ConfigMatchLine is a single matching line inside a switch's running-config.
type ConfigMatchLine struct {
	Line int    `json:"line"` // 1-based line number
	Text string `json:"text"`
}

// ConfigSearchHit groups all matching running-config lines for one switch.
type ConfigSearchHit struct {
	SwitchID   string            `json:"switchId"`
	SwitchName string            `json:"switchName"`
	Host       string            `json:"host"`
	Site       string            `json:"site,omitempty"`
	Family     string            `json:"family,omitempty"`
	TotalLines int               `json:"totalLines"`
	Lines      []ConfigMatchLine `json:"lines"`
}

// SearchConfigs searches every collected switch's in-memory running-config with
// zero SSH calls. site="" searches all sites; site="__none__" searches switches
// without a site.
func (m *Manager) SearchConfigs(query string, useRegex bool, site string) ([]ConfigSearchHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	var re *regexp.Regexp
	var qLow string
	if useRegex {
		var err error
		re, err = regexp.Compile("(?i)" + q)
		if err != nil {
			return nil, fmt.Errorf("invalid regular expression: %w", err)
		}
	} else {
		qLow = strings.ToLower(q)
	}

	sws := m.st.Switches()
	m.mu.Lock()
	defer m.mu.Unlock()

	const maxSnippets = 25
	var hits []ConfigSearchHit
	for _, sw := range sws {
		swSite := strings.TrimSpace(sw.Site)
		if site == "__none__" {
			if swSite != "" {
				continue
			}
		} else if site != "" && swSite != site {
			continue
		}
		snap := m.snaps[sw.ID]
		if snap == nil || snap.RunningConfig == "" {
			continue
		}
		var matched []ConfigMatchLine
		total := 0
		for idx, rawLine := range strings.Split(snap.RunningConfig, "\n") {
			line := strings.TrimRight(rawLine, "\r")
			var ok bool
			if re != nil {
				ok = re.MatchString(line)
			} else {
				ok = strings.Contains(strings.ToLower(line), qLow)
			}
			if !ok {
				continue
			}
			total++
			if len(matched) < maxSnippets {
				matched = append(matched, ConfigMatchLine{Line: idx + 1, Text: line})
			}
		}
		if total == 0 {
			continue
		}
		hits = append(hits, ConfigSearchHit{
			SwitchID:   sw.ID,
			SwitchName: displayName(sw, snap),
			Host:       sw.Host,
			Site:       swSite,
			Family:     snap.Family.Label(),
			TotalLines: total,
			Lines:      matched,
		})
	}
	sort.Slice(hits, func(i, j int) bool {
		return strings.ToLower(hits[i].SwitchName) < strings.ToLower(hits[j].SwitchName)
	})
	return hits, nil
}
