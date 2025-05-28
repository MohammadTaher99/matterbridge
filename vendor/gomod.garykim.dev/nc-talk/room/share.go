package room

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"

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
	// Create a share with the room
	req := t.User.RequestClient(request.Client{
		URL: constants.FilesSharingEndpoint + "shares",
		Method: "POST",
		Params: map[string]string{
			"shareType": "10", // 10 = Talk conversation
			"path": path,
			"shareWith": t.Token,
		},
		Header: map[string]string{
			"OCS-APIRequest": "true",
		},
	})
	resp, err := req.Do()
	if err != nil {
		return "", fmt.Errorf("failed to create share: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("failed to create share: unexpected status code: %d", resp.StatusCode())
	}

	// Parse the response
	var responseData map[string]interface{}
	err = json.Unmarshal(resp.Data, &responseData)
	if err != nil {
		return "", fmt.Errorf("failed to parse share response: %v", err)
	}

	// Extract the OCS data
	ocsData, ok := responseData["ocs"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format: no OCS data")
	}

	// Extract the data field
	data, ok := ocsData["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format: data is not an object")
	}

	// Get the file ID
	fileID, ok := data["id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response format: no file ID in share data")
	}

	// Create the message with the file ID
	message := fmt.Sprintf("image:%s", fileID)
	
	// Send the message to the chat
	chatReq := t.User.RequestClient(request.Client{
		URL: fmt.Sprintf("%s/ocs/v2.php/apps/spreed/api/v1/chat/%s", t.User.NextcloudURL, t.Token),
		Method: "POST",
		Body: []byte(fmt.Sprintf(`{
			"message": "%s",
			"actorType": "users",
			"actorId": "%s",
			"referenceId": "",
			"silent": false,
			"messageType": "comment",
			"parent": {"type": "message", "id": ""},
			"objectType": "chat",
			"objectId": "%s",
			"verb": "comment"
		}`, message, t.User.User, t.Token)),
		Header: map[string]string{
			"Content-Type": "application/json",
			"OCS-APIRequest": "true",
		},
	})
	chatResp, err := chatReq.Do()
	if err != nil {
		return "", fmt.Errorf("failed to send media message: %v", err)
	}
	if chatResp.StatusCode() != http.StatusOK && chatResp.StatusCode() != http.StatusCreated {
		return "", fmt.Errorf("failed to send media message: unexpected status code: %d", chatResp.StatusCode())
	}

	return fileID, nil
}
