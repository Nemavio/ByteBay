package smart

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Info struct {
	Device    string            `json:"device"`
	Healthy   bool              `json:"healthy"`
	TempC     *int              `json:"temp_c,omitempty"`
	Model     string            `json:"model,omitempty"`
	Serial    string            `json:"serial,omitempty"`
	PowerOnH  *int              `json:"power_on_hours,omitempty"`
	Attrs     map[string]string `json:"attributes,omitempty"`
	Raw       string            `json:"raw_output,omitempty"`
	Available bool              `json:"available"`
}

func Query(device string) (*Info, error) {
	dev := device
	if !strings.HasPrefix(dev, "/dev/") {
		dev = "/dev/" + dev
	}

	out, err := exec.Command("smartctl", "-a", "-j", dev).CombinedOutput()
	info := &Info{Device: dev, Available: true}

	// smartctl exits 4/8 on threshold exceeded but still returns JSON
	if len(out) == 0 {
		return nil, fmt.Errorf("smartctl: %w", err)
	}

	var raw map[string]any
	if jerr := json.Unmarshal(out, &raw); jerr != nil {
		info.Available = false
		info.Raw = string(out)
		return info, fmt.Errorf("smartctl json: %w (output: %s)", jerr, truncate(string(out), 200))
	}

	info.Healthy = jsonBool(raw, "smart_status", "passed")
	if v, ok := raw["temperature"].(map[string]any); ok {
		if t, ok := v["current"].(float64); ok {
			i := int(t)
			info.TempC = &i
		}
	}
	if v, ok := raw["model_name"].(string); ok {
		info.Model = v
	}
	if v, ok := raw["serial_number"].(string); ok {
		info.Serial = v
	}
	if v, ok := raw["power_on_time"].(map[string]any); ok {
		if h, ok := v["hours"].(float64); ok {
			i := int(h)
			info.PowerOnH = &i
		}
	}

	attrs := make(map[string]string)
	if ata, ok := raw["ata_smart_attributes"].(map[string]any); ok {
		if table, ok := ata["table"].([]any); ok {
			for _, row := range table {
				m, _ := row.(map[string]any)
				name, _ := m["name"].(string)
				val, _ := m["raw"].(map[string]any)
				if s, ok := val["string"].(string); ok && name != "" {
					attrs[name] = s
				}
			}
		}
	}
	if len(attrs) > 0 {
		info.Attrs = attrs
	}
	return info, nil
}

func jsonBool(m map[string]any, key, sub string) bool {
	v, ok := m[key].(map[string]any)
	if !ok {
		return true
	}
	passed, _ := v[sub].(bool)
	return passed
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
