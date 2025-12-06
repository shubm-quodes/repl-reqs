package cmd

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/shubm-quodes/repl-reqs/util"
)

type Step struct {
	Name            string   `json:"name"`
	Cmd             []string `json:"cmd"`
	sequenceErrChan chan error
	uChan           chan TaskStatus
	Task            TaskUpdater
	ParentStep      *Step
	HasFailed       bool
}

var (
	expansionRegex     = regexp.MustCompile(`\{\{([^}]+)\}\}`)
	stepExpansionRegex = regexp.MustCompile(`^\$(\d+)\.(.+)$`)
)

func (s *Step) ExpandTokens(seq Sequence, variables map[string]string) ([]string, error) {
	expandedCmd := make([]string, len(s.Cmd))

	for i, token := range s.Cmd {
		expanded, err := s.expandToken(token, seq, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to expand token '%s': %w", token, err)
		}
		expandedCmd[i] = expanded
	}

	return expandedCmd, nil
}

func (s *Step) expandToken(
	token string,
	seq Sequence,
	variables map[string]string,
) (string, error) {
	matches := expansionRegex.FindAllStringSubmatch(token, -1)

	if len(matches) == 0 {
		return token, nil
	}

	result := token
	for _, match := range matches {
		fullMatch := match[0]
		content := match[1]

		var replacement string
		var err error

		if stepExpansionRegex.MatchString(content) {
			replacement, err = s.expandStepBased(content, seq)
		} else {
			replacement, err = s.expandVariable(content, variables)
		}

		if err != nil {
			return "", err
		}

		result = strings.Replace(result, fullMatch, replacement, 1)
	}

	return result, nil
}

func (s *Step) expandVariable(varName string, variables map[string]string) (string, error) {
	if val, ok := variables[varName]; ok {
		return val, nil
	}
	return "", fmt.Errorf("variable '%s' not found", varName)
}

func (s *Step) expandStepBased(content string, seq Sequence) (string, error) {
	submatches := stepExpansionRegex.FindStringSubmatch(content)
	if len(submatches) != 3 {
		return "", fmt.Errorf("invalid step expansion format: %s", content)
	}

	stepNum, err := strconv.Atoi(submatches[1])
	if err != nil {
		return "", fmt.Errorf("invalid step number: %s", submatches[1])
	}

	// Convert to 0-based index
	stepIndex := stepNum - 1

	if stepIndex < 0 || stepIndex >= len(seq) {
		return "", fmt.Errorf("step %d is out of range (sequence has %d steps)", stepNum, len(seq))
	}

	targetStep := seq[stepIndex]

	if targetStep.Task == nil || targetStep.Task.GetResult() == nil {
		return "", fmt.Errorf("step %d has no result available", stepNum)
	}

	path := submatches[2]

	// Check if there's a filter condition (contains '=')
	if strings.Contains(path, "=") {
		return s.expandWithFilter(targetStep.Task.GetResult().(*http.Response), path)
	}

	// Simple value extraction
	return s.extractValue(targetStep.Task.GetResult().(*http.Response), path)
}

func (s *Step) expandWithFilter(resp *http.Response, path string) (string, error) {
	parts := strings.SplitN(path, "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid filter format: %s", path)
	}

	leftPart := parts[0]
	rightPart := parts[1]

	var filterValue string
	var fieldToExtract string

	if strings.Contains(rightPart, ".") {
		rightParts := strings.SplitN(rightPart, ".", 2)
		filterValue = rightParts[0]
		fieldToExtract = rightParts[1]
	} else {
		filterValue = rightPart
		fieldToExtract = ""
	}

	pathParts := strings.Split(leftPart, ".")

	data, err := decodeResponse(resp)
	if err != nil {
		return "", err
	}

	// Navigate to the array
	current := data
	for i := 0; i < len(pathParts)-1; i++ {
		current, err = util.NavigateToKey(current, pathParts[i])
		if err != nil {
			return "", err
		}
	}

	// The last part is the property to filter on
	propertyName := pathParts[len(pathParts)-1]

	values, err := filterArray(current, propertyName, filterValue, fieldToExtract)
	if err != nil {
		return "", err
	}

	return strings.Join(values, ","), nil
}

// extractValue extracts a simple value from the response
func (s *Step) extractValue(resp *http.Response, path string) (string, error) {
	data, err := decodeResponse(resp)
	if err != nil {
		return "", err
	}

	pathParts := strings.Split(path, ".")
	current := data

	for _, part := range pathParts {
		current, err = util.NavigateToKey(current, part)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%v", current), nil
}

func decodeResponse(resp *http.Response) (any, error) {
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	var data any

	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to decode JSON: %w", err)
		}
	} else if strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml") {
		if err := xml.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to decode XML: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	return data, nil
}

func formatResult(data any) string {
	switch v := data.(type) {
	case []any:
		// If it's an array of values, join them
		var strs []string
		for _, item := range v {
			strs = append(strs, fmt.Sprintf("%v", item))
		}
		return strings.Join(strs, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// filterArray filters an array based on a property and value
// If fieldToExtract is provided, extracts that field from matching objects
// Otherwise returns the matched property value itself
func filterArray(
	data any,
	propertyName, filterValue, fieldToExtract string,
) ([]string, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array but got %T", data)
	}

	var results []string

	for _, item := range arr {
		// Skip non-object elements in mixed arrays
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		// Check if the filter property exists and matches
		if val, exists := obj[propertyName]; exists {
			if fmt.Sprintf("%v", val) == filterValue {
				// If fieldToExtract is specified, get that field from the object
				if fieldToExtract != "" {
					if extractedVal, fieldExists := obj[fieldToExtract]; fieldExists {
						results = append(results, fmt.Sprintf("%v", extractedVal))
					} else {
						return nil, fmt.Errorf("field '%s' not found in matching object", fieldToExtract)
					}
				} else {
					// Otherwise, return the matching property value
					results = append(results, fmt.Sprintf("%v", val))
				}
			}
		}
	}

	if len(results) == 0 {
		if fieldToExtract != "" {
			return nil, fmt.Errorf(
				"no matching items found for %s=%s with field %s",
				propertyName,
				filterValue,
				fieldToExtract,
			)
		}
		return nil, fmt.Errorf("no matching items found for %s=%s", propertyName, filterValue)
	}

	return results, nil
}

// GetCmd returns the command slice
func (s *Step) GetCmd() []string {
	return s.Cmd
}

// SetCmd sets the command slice
func (s *Step) SetCmd(cmd []string) {
	s.Cmd = cmd
}

// GetName returns the step name
func (s *Step) GetName() string {
	return s.Name
}

// Blocks and waits for updates from the underlying step cmd
func (s *Step) watchForUpdates(originalTask TaskUpdater) {
	u := <-s.uChan //block until complete
	if u.Error != nil {
		s.sequenceErrChan <- fmt.Errorf(
			"sequence step %s failed. failed to exec cmd %s: %s",
			s.GetName(),
			strings.Join(s.GetCmd(), " "),
			u.Error.Error(),
		)
		s.HasFailed = true
		originalTask.AppendOutput(u.Output)
		originalTask.Fail(u.Error)
	} else {
		originalTask.AppendOutput(u.Output)
		originalTask.CompleteWithMessage(u.Message, u.Result)
	}
}
