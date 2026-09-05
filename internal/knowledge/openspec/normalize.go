package openspec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/SBakolis/north/internal/knowledge"
)

var (
	headingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	taskPattern    = regexp.MustCompile(`^\s*[-*]\s+\[([ xX])\]\s+(.+?)\s*$`)
)

func normalizeMarkdown(snapshot *knowledge.Snapshot, artifactID, source, content string) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var section string
	var title string
	for index, line := range lines {
		if match := headingPattern.FindStringSubmatch(line); match != nil {
			section = strings.TrimSpace(match[2])
			if title == "" && len(match[1]) == 1 {
				title = section
			}
			if kind, name := semanticHeading(section); kind != "" {
				text := collectSection(lines, index+1, len(match[1]))
				src := fmt.Sprintf("%s#L%d", source, index+1)
				switch kind {
				case "requirement":
					snapshot.Requirements = append(snapshot.Requirements, knowledge.Requirement{ID: stableID(artifactID, "requirement", len(snapshot.Requirements)+1), Text: joinName(name, text), Source: src})
				case "decision":
					snapshot.DesignDecisions = append(snapshot.DesignDecisions, knowledge.DesignDecision{ID: stableID(artifactID, "decision", len(snapshot.DesignDecisions)+1), Text: joinName(name, text), Source: src})
				case "criterion":
					snapshot.AcceptanceCriteria = append(snapshot.AcceptanceCriteria, knowledge.Criterion{ID: stableID(artifactID, "criterion", len(snapshot.AcceptanceCriteria)+1), Text: joinName(name, text), Source: src})
				}
			}
		}
		if match := taskPattern.FindStringSubmatch(line); match != nil {
			snapshot.Tasks = append(snapshot.Tasks, knowledge.Task{
				ID: stableID(artifactID, "task", len(snapshot.Tasks)+1), Text: strings.TrimSpace(match[2]),
				Source: fmt.Sprintf("%s#L%d", source, index+1), Completed: strings.TrimSpace(match[1]) != "",
			})
		}
	}
	if snapshot.Change == nil && title != "" {
		summary := firstUsefulSection(lines)
		snapshot.Change = &knowledge.ChangeDescription{Title: title, Summary: summary}
	}
}

func semanticHeading(heading string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(heading))
	for _, prefix := range []string{"requirement:", "requirement "} {
		if strings.HasPrefix(lower, prefix) {
			return "requirement", strings.TrimSpace(heading[len(prefix):])
		}
	}
	for _, prefix := range []string{"scenario:", "acceptance criterion:", "criterion:"} {
		if strings.HasPrefix(lower, prefix) {
			return "criterion", strings.TrimSpace(heading[len(prefix):])
		}
	}
	for _, prefix := range []string{"decision:", "design decision:", "adr:"} {
		if strings.HasPrefix(lower, prefix) {
			return "decision", strings.TrimSpace(heading[len(prefix):])
		}
	}
	return "", ""
}

func collectSection(lines []string, start, level int) string {
	var result []string
	for _, line := range lines[start:] {
		if match := headingPattern.FindStringSubmatch(line); match != nil && len(match[1]) <= level {
			break
		}
		if taskPattern.MatchString(line) {
			continue
		}
		result = append(result, strings.TrimSpace(line))
	}
	return strings.TrimSpace(strings.Join(compact(result), " "))
}

func compact(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func firstUsefulSection(lines []string) string {
	var body []string
	for _, line := range lines[1:] {
		if headingPattern.MatchString(line) && len(body) > 0 {
			break
		}
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "<!--") {
			body = append(body, strings.TrimSpace(line))
		}
	}
	return strings.TrimSpace(strings.Join(body, " "))
}

func stableID(artifact, kind string, index int) string {
	return fmt.Sprintf("%s:%s:%d", artifact, kind, index)
}

func joinName(name, text string) string {
	if name == "" {
		return text
	}
	if text == "" {
		return name
	}
	return name + ": " + text
}

func mergeShow(snapshot *knowledge.Snapshot, show showOutput) {
	if snapshot.Change == nil {
		snapshot.Change = &knowledge.ChangeDescription{Title: show.Title, Summary: show.Why}
	} else {
		if snapshot.Change.Title == "" {
			snapshot.Change.Title = show.Title
		}
		if snapshot.Change.Summary == "" {
			snapshot.Change.Summary = show.Why
		}
	}
	for _, delta := range show.Deltas {
		text := strings.TrimSpace(delta.Description)
		if text == "" || requirementExists(snapshot.Requirements, text) {
			continue
		}
		snapshot.Requirements = append(snapshot.Requirements, knowledge.Requirement{
			ID:   "delta:" + strings.ToLower(delta.Operation) + ":" + delta.Spec,
			Text: text, Source: "openspec show " + show.ID + "#" + delta.Spec,
		})
	}
}

func requirementExists(requirements []knowledge.Requirement, text string) bool {
	for _, requirement := range requirements {
		if strings.Contains(requirement.Text, text) || strings.Contains(text, requirement.Text) {
			return true
		}
	}
	return false
}
