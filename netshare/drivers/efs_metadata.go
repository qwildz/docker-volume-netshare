package drivers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	MetaDataURL = "http://169.254.169.254/latest/dynamic/instance-identity/document"
)

type metaData struct {
	AvailZone string `json:"availabilityZone,omitempty"`
	Region    string `json:"region,omitempty"`
}

func fetchAWSMetaData() (*metaData, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	r, err := client.Get(MetaDataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch AWS metadata (timeout 5s): %w", err)
	}
	defer r.Body.Close()

	md := &metaData{}
	if err := json.NewDecoder(r.Body).Decode(md); err != nil {
		return nil, fmt.Errorf("failed to decode AWS metadata: %w", err)
	}
	return md, nil
}
