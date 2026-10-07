package bathing

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// DatahubWatcher is assigned by datahub.Watch in an init function.
// This indirection breaks the circular import between bathing and bathing/datahub.
var DatahubWatcher func(context.Context, string, string, time.Duration, *http.Client) (bool, error)

// watchDatahubEdition checks if a newer edition of the Datahub is available.
// It delegates to the datahub.Watch function via the DatahubWatcher variable.
func watchDatahubEdition(ctx context.Context, currentURL string, timeout time.Duration) error {
	if DatahubWatcher == nil {
		return nil // datahub watcher not initialized
	}

	// Parse the URL to extract the host for validation
	u, err := url.Parse(currentURL)
	if err != nil {
		return err
	}
	allowedHost := u.Hostname()

	_, err = DatahubWatcher(ctx, currentURL, allowedHost, timeout, &http.Client{})
	return err
}
