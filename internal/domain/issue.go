package domain

type Issue struct {
	Category string `json:"category,omitempty"`
	Code     string `json:"code"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

func issue(code, field, message string) Issue {
	return Issue{Category: issueCategory(field), Code: code, Field: field, Message: message}
}

func issueCategory(field string) string {
	switch field {
	case "workers", "attendant", "worker_ids", "attendant_id":
		return "PERSONNEL_ROLES"
	case "gas_readings":
		return "GAS_TESTING"
	case "isolation_points", "confirmed_isolation_point_ids":
		return "ISOLATION_POINTS"
	case "control_measures", "confirmed_control_measure_ids":
		return "CONTROL_MEASURES"
	default:
		return "COMPLETENESS"
	}
}
