package pan

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type uploadInitWire struct {
	apiResponse
	Data struct {
		Status    int             `json:"status"`
		SignKey   string          `json:"sign_key"`
		SignCheck string          `json:"sign_check"`
		Bucket    string          `json:"bucket"`
		Object    string          `json:"object"`
		Callback  json.RawMessage `json:"callback"`
	} `json:"data"`
}

type uploadTokenWire struct {
	apiResponse
	Data struct {
		Endpoint      string `json:"endpoint"`
		AccessKeyID   string `json:"AccessKeyId"`
		AccessSecret  string `json:"AccessKeySecret"`
		SecurityToken string `json:"SecurityToken"`
	} `json:"data"`
}

// UploadFile stores small content (NFO, artwork) in a 115 directory through
// the open platform upload flow: init (with rapid-upload and sign-challenge
// handling), then one OSS PutObject with the STS credentials from
// /open/upload/get_token. The OSS request is signed by hand so the client
// needs no storage SDK.
func (client *Client) UploadFile(ctx context.Context, accessToken, dirID, name string, content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("upload %s: empty content", name)
	}
	form := map[string]string{
		"file_name": name,
		"file_size": strconv.Itoa(len(content)),
		"target":    "U_1_" + dirID,
		"fileid":    SHA1(content),
		"preid":     SHA1(content[:min(len(content), 128*1024)]),
	}
	init, err := client.uploadInit(ctx, accessToken, form)
	if err != nil {
		return err
	}
	// Statuses 6-8 are the rapid-upload proof-of-possession challenge: hash
	// the requested byte range and re-init with the answer.
	if init.Data.Status >= 6 && init.Data.Status <= 8 {
		start, end, err := parseSignCheck(init.Data.SignCheck, len(content))
		if err != nil {
			return err
		}
		form["sign_key"] = init.Data.SignKey
		form["sign_val"] = SHA1(content[start : end+1])
		init, err = client.uploadInit(ctx, accessToken, form)
		if err != nil {
			return err
		}
	}
	// Status 2: the content already exists on 115 (rapid upload), nothing
	// further to send.
	if init.Data.Status == 2 {
		return nil
	}
	if init.Data.Bucket == "" || init.Data.Object == "" {
		return fmt.Errorf("upload %s: 115 returned unsupported status %d", name, init.Data.Status)
	}
	token, err := client.uploadToken(ctx, accessToken)
	if err != nil {
		return err
	}
	callback, callbackVar := splitUploadCallback(init.Data.Callback)
	return ossPutObject(ctx, client.media, token, init.Data.Bucket, init.Data.Object, callback, callbackVar, content)
}

func (client *Client) uploadInit(ctx context.Context, accessToken string, form map[string]string) (uploadInitWire, error) {
	return apiRequest[uploadInitWire](
		client,
		client.http.R().SetContext(ctx).SetAuthToken(accessToken).SetFormData(form),
		http.MethodPost,
		apiURL+"/open/upload/init",
		"upload init",
	)
}

func (client *Client) uploadToken(ctx context.Context, accessToken string) (uploadTokenWire, error) {
	return apiRequest[uploadTokenWire](
		client,
		client.http.R().SetContext(ctx).SetAuthToken(accessToken),
		http.MethodGet,
		apiURL+"/open/upload/get_token",
		"upload token",
	)
}

func parseSignCheck(signCheck string, size int) (int, int, error) {
	startText, endText, ok := strings.Cut(signCheck, "-")
	if !ok {
		return 0, 0, fmt.Errorf("115 sign_check %q is malformed", signCheck)
	}
	start, err1 := strconv.Atoi(startText)
	end, err2 := strconv.Atoi(endText)
	if err1 != nil || err2 != nil || start < 0 || end < start || end >= size {
		return 0, 0, fmt.Errorf("115 sign_check %q is out of range for %d bytes", signCheck, size)
	}
	return start, end, nil
}

// splitUploadCallback separates the callback document 115 hands out at init
// into the two base64 payloads the OSS request carries as headers.
func splitUploadCallback(raw json.RawMessage) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}
	var doc struct {
		Callback  string `json:"callback"`
		Variables string `json:"callback_var"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Callback == "" {
		return "", ""
	}
	return base64.StdEncoding.EncodeToString([]byte(doc.Callback)),
		base64.StdEncoding.EncodeToString([]byte(doc.Variables))
}

// ossPutObject stores content at bucket/object on the STS endpoint and asks
// OSS to deliver the 115 callback that registers the uploaded file.
func ossPutObject(ctx context.Context, httpClient *http.Client, token uploadTokenWire, bucket, object, callback, callbackVar string, content []byte) error {
	endpoint := strings.TrimRight(token.Data.Endpoint, "/")
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse OSS endpoint: %w", err)
	}
	parsed.Host = bucket + "." + parsed.Host
	parsed.Path = "/" + escapeObjectKey(object)
	target := parsed.String()

	date := time.Now().UTC().Format(http.TimeFormat)
	headers := map[string]string{
		"x-oss-security-token": token.Data.SecurityToken,
	}
	if callback != "" {
		headers["x-oss-callback"] = callback
		headers["x-oss-callback-var"] = callbackVar
	}
	signature := ossSignature(token.Data.AccessSecret, date, headers, bucket, object)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", "OSS "+token.Data.AccessKeyID+":"+signature)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("OSS put object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("OSS put object: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// ossSignature computes the Aliyun OSS V1 signature for a PutObject with no
// Content-MD5 and no Content-Type: PUT\n\n\n<date>\n<canonical x-oss-*>​/<bucket>/<object>.
func ossSignature(secret, date string, headers map[string]string, bucket, object string) string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for _, key := range keys {
		canonical.WriteString(key + ":" + headers[key] + "\n")
	}
	stringToSign := "PUT\n\n\n" + date + "\n" + canonical.String() + "/" + bucket + "/" + object
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func escapeObjectKey(object string) string {
	segments := strings.Split(object, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
