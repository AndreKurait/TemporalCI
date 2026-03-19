package activities

import "strings"

// MaskSecrets replaces secret values in output with ***.
func MaskSecrets(output string, secrets map[string]string) string {
	if len(secrets) == 0 {
		return output
	}
	for _, v := range secrets {
		if len(v) > 3 { // don't mask very short values like "true"
			output = strings.ReplaceAll(output, v, "***")
		}
	}
	return output
}
