package room

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"

	"github.com/monaco-io/request"

	"gomod.garykim.dev/nc-talk/constants"
)

// WebDAVResponse represents the XML response from WebDAV PROPFIND
type WebDAVResponse struct {
	XMLName xml.Name `xml:"multistatus"`
	Response []struct {
		Href string `xml:"href"`
		Propstat []struct {
			Prop struct {
				LastModified string `xml:"getlastmodified"`
				ContentLength string `xml:"getcontentlength"`
				ResourceType string `xml:"resourcetype"`
				ETag string `xml:"getetag"`
				ContentType string `xml:"getcontenttype"`
			} `xml:"prop"`
			Status string `xml:"status"`
		} `xml:"propstat"`
	} `xml:"response"`
}

// ShareFile shares the file at the given path with the talk room
func (t *TalkRoom) ShareFile(path string) (string, error) {
	// Create a public share
	req := t.User.RequestClient(request.Client{
		URL: constants.FilesSharingEndpoint + "shares",
		Params: map[string]string{
			"shareType": "3", // Public link share
			"path":      path,
			"permissions": "1", // Read-only permission
			"expireDate": "",  // No expiration
			"publicUpload": "false", // Don't allow public uploads
		},
	})
	resp, err := req.Do()
	if err != nil {
		return "", fmt.Errorf("failed to create public share: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("failed to create public share: unexpected status code: %d", resp.StatusCode())
	}

	// Debug: Print raw response
	fmt.Printf("Share response raw: %s\n", string(resp.Data))

	// Parse the response as a generic map to handle both array and object formats
	var responseData map[string]interface{}
	err = json.Unmarshal(resp.Data, &responseData)
	if err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
		return "", err
	}

	// Extract the OCS data
	ocsData, ok := responseData["ocs"].(map[string]interface{})
	if !ok {
		fmt.Printf("No OCS data found in response. Response data: %+v\n", responseData)
		return "", fmt.Errorf("invalid response format: no OCS data")
	}

	// Debug: Print OCS data structure
	fmt.Printf("OCS data structure: %+v\n", ocsData)

	// Extract the data field which could be an array or object
	data, ok := ocsData["data"].([]interface{})
	if !ok {
		// Try to handle it as a single object instead
		if singleData, ok := ocsData["data"].(map[string]interface{}); ok {
			if url, ok := singleData["url"].(string); ok {
				fmt.Printf("Found URL in single object response: %s\n", url)
				return url, nil
			}
		}
		fmt.Printf("Data is not an array or single object. Data type: %T, value: %+v\n", ocsData["data"], ocsData["data"])
		return "", fmt.Errorf("invalid response format: data is not in expected format")
	}

	if len(data) == 0 {
		fmt.Printf("Empty data array. Full OCS response: %+v\n", ocsData)
		// Try to create a share URL manually since we have the path
		parsedURL, err := url.Parse(t.User.NextcloudURL)
		if err != nil {
			return "", fmt.Errorf("failed to parse base URL: %v", err)
		}

		// Try to get the file info first
		fileReq := t.User.RequestClient(request.Client{
			URL: t.User.NextcloudURL + "/remote.php/dav/files/" + t.User.User + path,
			Method: "PROPFIND",
			Header: map[string]string{
				"Depth": "0",
			},
		})
		fileResp, err := fileReq.Do()
		if err != nil {
			return "", fmt.Errorf("failed to get file info: %v", err)
		}

		// Debug: Print WebDAV response
		fmt.Printf("WebDAV response: %s\n", string(fileResp.Data))

		// Parse the XML response
		var davResp WebDAVResponse
		err = xml.Unmarshal(fileResp.Data, &davResp)
		if err != nil {
			return "", fmt.Errorf("failed to parse WebDAV response: %v", err)
		}

		if len(davResp.Response) == 0 || len(davResp.Response[0].Propstat) == 0 {
			return "", fmt.Errorf("no file info in WebDAV response")
		}

		// Create a public share with a token
		shareReq := t.User.RequestClient(request.Client{
			URL: constants.FilesSharingEndpoint + "shares",
			Method: "POST",
			Params: map[string]string{
				"shareType": "3", // Public link share
				"path":      path,
				"permissions": "1", // Read-only permission
				"expireDate": "",  // No expiration
				"publicUpload": "false", // Don't allow public uploads
			},
		})
		shareResp, err := shareReq.Do()
		if err != nil {
			return "", fmt.Errorf("failed to create share: %v", err)
		}

		// Parse the share response
		var shareData map[string]interface{}
		err = json.Unmarshal(shareResp.Data, &shareData)
		if err != nil {
			return "", fmt.Errorf("failed to parse share response: %v", err)
		}

		// Extract the share token
		shareInfo, ok := shareData["ocs"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("invalid share response format")
		}

		shareData, ok = shareInfo["data"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("invalid share data format")
		}

		token, ok := shareData["token"].(string)
		if !ok {
			return "", fmt.Errorf("no share token in response")
		}

		// Construct the share URL using the token
		shareURL := fmt.Sprintf("%s://%s/index.php/s/%s", parsedURL.Scheme, parsedURL.Host, token)
		fmt.Printf("Generated share URL: %s\n", shareURL)
		return shareURL, nil
	}

	// Get the first item from the array
	firstItem, ok := data[0].(map[string]interface{})
	if !ok {
		fmt.Printf("First item is not an object. Item type: %T, value: %+v\n", data[0], data[0])
		return "", fmt.Errorf("invalid response format: first item is not an object")
	}

	// Extract the URL
	url, ok := firstItem["url"].(string)
	if !ok {
		fmt.Printf("No URL found in first item. Item data: %+v\n", firstItem)
		return "", fmt.Errorf("invalid response format: no URL in share data")
	}

	fmt.Printf("Successfully extracted URL: %s\n", url)
	return url, nil
}
