package _123

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/go-resty/resty/v2"
)

// Share creation and management for 123pan
const (
	ShareCreate = MainApi + "/share/create"
	ShareInfoAPI   = MainApi + "/share/info"
)

// ShareCreateReq represents the request to create a share
type ShareCreateReq struct {
	FileIDList   []int64 `json:"fileIdList"`
	SharePwd     string  `json:"sharePwd,omitempty"`
	ShareName    string  `json:"shareName,omitempty"`
	ExpireTime   int64   `json:"expireTime,omitempty"` // 0 for permanent, otherwise unix timestamp
	OnlyShowLink bool    `json:"onlyShowLink"`
}

// ShareCreateResp represents the response from creating a share
type ShareCreateResp struct {
	Data struct {
		ShareKey string `json:"shareKey"`
		SharePwd string `json:"sharePwd"`
	} `json:"data"`
}

// Pan123ShareInfo represents share information
type Pan123ShareInfo struct {
	ShareKey   string    `json:"shareKey"`
	SharePwd   string    `json:"sharePwd"`
	ShareURL   string    `json:"shareUrl"`
	ExpireTime time.Time `json:"expireTime"`
}

// CreateShare creates a share link for the given file IDs
func (d *Pan123) CreateShare(ctx context.Context, fileIDs []int64, pwd string, expireDays int) (*Pan123ShareInfo, error) {
	if len(fileIDs) == 0 {
		return nil, fmt.Errorf("fileIDs cannot be empty")
	}

	var expireTime int64
	if expireDays > 0 {
		expireTime = time.Now().Add(time.Duration(expireDays) * 24 * time.Hour).Unix()
	}

	req := ShareCreateReq{
		FileIDList:   fileIDs,
		SharePwd:     pwd,
		OnlyShowLink: false,
		ExpireTime:   expireTime,
	}

	var resp ShareCreateResp
	_, err := d.Request(ShareCreate, http.MethodPost, func(r *resty.Request) {
		r.SetBody(req)
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	shareURL := fmt.Sprintf("https://www.123pan.com/s/%s", resp.Data.ShareKey)
	
	info := &Pan123ShareInfo{
		ShareKey: resp.Data.ShareKey,
		SharePwd: resp.Data.SharePwd,
		ShareURL: shareURL,
	}
	
	if expireTime > 0 {
		info.ExpireTime = time.Unix(expireTime, 0)
	}

	return info, nil
}

// GetShareLink returns the share link for a file, creating one if necessary
func (d *Pan123) GetShareLink(ctx context.Context, file model.Obj, pwd string, expireDays int) (string, error) {
	if f, ok := file.(File); ok {
		fileIDs := []int64{f.FileId}
		info, err := d.CreateShare(ctx, fileIDs, pwd, expireDays)
		if err != nil {
			return "", err
		}
		
		// Add password to URL if present
		shareURL := info.ShareURL
		if pwd != "" {
			shareURL += "?pwd=" + pwd
		}
		
		return shareURL, nil
	}
	return "", fmt.Errorf("invalid file object")
}

// Other method implementation to support share creation
func (d *Pan123) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	switch args.Method {
	case "create_share":
		// Extract parameters from args.Data
		pwd := ""
		expireDays := 0
		
		if dataMap, ok := args.Data.(map[string]interface{}); ok {
			if pwdVal, ok := dataMap["password"].(string); ok {
				pwd = pwdVal
			}
			if daysVal, ok := dataMap["expire_days"].(float64); ok {
				expireDays = int(daysVal)
			}
		}
		
		return d.GetShareLink(ctx, args.Obj, pwd, expireDays)
	default:
		return nil, fmt.Errorf("unsupported method: %s", args.Method)
	}
}
