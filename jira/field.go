package jira

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

// Fields contains the standard fields of a Jira issue.
type Fields struct {
	Summary string `json:"summary"`
	Status  struct {
		Name string `json:"name"`
	} `json:"status"`
	Issuetype struct {
		Name string `json:"name"`
	} `json:"issuetype"`
	Assignee struct {
		DisplayName string `json:"displayName"`
	} `json:"assignee"`
	Created      string                 `json:"created"`
	Updated      string                 `json:"updated"`
	CustomFields map[string]interface{} `json:"-"` // Will be populated from raw JSON.
}

// UnmarshalJSON custom unmarshals Fields to extract custom fields.
func (f *Fields) UnmarshalJSON(data []byte) error {
	// First unmarshal into a map to capture all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.WithStack(err)
	}

	// Extract known fields
	if summary, ok := raw["summary"].(string); ok {
		f.Summary = summary
	}
	if status, ok := raw["status"].(map[string]interface{}); ok {
		if name, ok := status["name"].(string); ok {
			f.Status.Name = name
		}
	}
	if issuetype, ok := raw["issuetype"].(map[string]interface{}); ok {
		if name, ok := issuetype["name"].(string); ok {
			f.Issuetype.Name = name
		}
	}
	if assignee, ok := raw["assignee"].(map[string]interface{}); ok {
		if displayName, ok := assignee["displayName"].(string); ok {
			f.Assignee.DisplayName = displayName
		}
	}
	if created, ok := raw["created"].(string); ok {
		f.Created = created
	}
	if updated, ok := raw["updated"].(string); ok {
		f.Updated = updated
	}

	// Extract custom fields
	f.CustomFields = make(map[string]interface{})
	for key, value := range raw {
		if strings.HasPrefix(key, "customfield_") {
			f.CustomFields[key] = value
		}
	}

	return nil
}
