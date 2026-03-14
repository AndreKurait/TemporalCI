package activities

// CloneInput defines the input for the CloneRepo activity.
type CloneInput struct {
	Repo string `json:"repo"`
	Ref  string `json:"ref"`
}

// CloneResult defines the output of the CloneRepo activity.
type CloneResult struct {
	Dir string `json:"dir"`
}
