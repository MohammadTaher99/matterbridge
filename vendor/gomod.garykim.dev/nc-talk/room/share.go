package room

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/monaco-io/request"

	"gomod.garykim.dev/nc-talk/constants"
)

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
	var responseData struct {
		OCS struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"ocs"`
	}
	err = json.Unmarshal(resp.Data, &responseData)
	if err != nil {
		return "", fmt.Errorf("failed to parse share response: %v", err)
	}

	// Get the file ID
	fileID := responseData.OCS.Data.ID
	if fileID == "" {
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
