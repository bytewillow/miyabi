package pan

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// EnsureDir returns the child directory of parentID named name, creating it
// when missing. Lookup is paginated: a parent may hold more entries than one
// list page returns.
func (client *Client) EnsureDir(ctx context.Context, accessToken, parentID, name string) (string, error) {
	if id, found, err := client.findDir(ctx, accessToken, parentID, name); err != nil || found {
		return id, err
	}
	type addWire struct {
		apiResponse
		Data struct {
			FileID string `json:"file_id"`
		} `json:"data"`
	}
	result, err := apiRequest[addWire](
		client,
		client.http.R().SetContext(ctx).SetAuthToken(accessToken).SetFormData(map[string]string{
			"pid": parentID, "file_name": name,
		}),
		http.MethodPost,
		apiURL+"/open/folder/add",
		"folder creation",
	)
	if err != nil {
		// A concurrent creator may have won the race; the directory existing
		// is the outcome we wanted either way.
		if id, found, ferr := client.findDir(ctx, accessToken, parentID, name); ferr == nil && found {
			return id, nil
		}
		return "", err
	}
	if result.Data.FileID != "" {
		return result.Data.FileID, nil
	}
	id, found, err := client.findDir(ctx, accessToken, parentID, name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("115 created folder %q but it cannot be found", name)
	}
	return id, nil
}

func (client *Client) findDir(ctx context.Context, accessToken, parentID, name string) (string, bool, error) {
	for offset := 0; ; offset += 100 {
		page, err := client.List(ctx, accessToken, parentID, offset, 100)
		if err != nil {
			return "", false, err
		}
		for _, entry := range page.Files {
			if entry.IsDirectory && entry.Name == name {
				return entry.ID, true, nil
			}
		}
		if !page.HasMore || len(page.Files) == 0 {
			return "", false, nil
		}
	}
}

// MoveFiles moves files into targetDirID. Moves preserve file IDs, so the
// library index stays valid; batches stay under the API's per-call limit.
func (client *Client) MoveFiles(ctx context.Context, accessToken, targetDirID string, fileIDs []string) error {
	for start := 0; start < len(fileIDs); start += 100 {
		end := min(start+100, len(fileIDs))
		_, err := apiRequest[apiResponse](
			client,
			client.http.R().SetContext(ctx).SetAuthToken(accessToken).SetFormData(map[string]string{
				"to_cid": targetDirID, "file_ids": strings.Join(fileIDs[start:end], ","),
			}),
			http.MethodPost,
			apiURL+"/open/ufile/move",
			"file move",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// RenameFile changes one file's display name in place.
func (client *Client) RenameFile(ctx context.Context, accessToken, fileID, name string) error {
	_, err := apiRequest[apiResponse](
		client,
		client.http.R().SetContext(ctx).SetAuthToken(accessToken).SetFormData(map[string]string{
			"file_id": fileID, "file_name": name,
		}),
		http.MethodPost,
		apiURL+"/open/ufile/update",
		"file rename",
	)
	return err
}
