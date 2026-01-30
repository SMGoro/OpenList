package op

import (
	"context"
	"fmt"
	stdpath "path"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/singleflight"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/OpenListTeam/go-cache"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func makeJoined(sdb []model.SharingDB) []model.Sharing {
	creator := make(map[uint]*model.User)
	return utils.MustSliceConvert(sdb, func(s model.SharingDB) model.Sharing {
		var c *model.User
		var ok bool
		if c, ok = creator[s.CreatorId]; !ok {
			var err error
			if c, err = GetUserById(s.CreatorId); err != nil {
				c = nil
			} else {
				creator[s.CreatorId] = c
			}
		}
		var files []string
		if err := utils.Json.UnmarshalFromString(s.FilesRaw, &files); err != nil {
			files = make([]string, 0)
		}
		return model.Sharing{
			SharingDB: &s,
			Files:     files,
			Creator:   c,
		}
	})
}

var sharingCache = cache.NewMemCache(cache.WithShards[*model.Sharing](8))
var sharingG singleflight.Group[*model.Sharing]

func GetSharingById(id string, refresh ...bool) (*model.Sharing, error) {
	if !utils.IsBool(refresh...) {
		if sharing, ok := sharingCache.Get(id); ok {
			log.Debugf("use cache when get sharing %s", id)
			return sharing, nil
		}
	}
	sharing, err, _ := sharingG.Do(id, func() (*model.Sharing, error) {
		s, err := db.GetSharingById(id)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed get sharing [%s]", id)
		}
		creator, err := GetUserById(s.CreatorId)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed get sharing creator [%s]", id)
		}
		var files []string
		if err = utils.Json.UnmarshalFromString(s.FilesRaw, &files); err != nil {
			files = make([]string, 0)
		}
		return &model.Sharing{
			SharingDB: s,
			Files:     files,
			Creator:   creator,
		}, nil
	})
	return sharing, err
}

func GetSharings(pageIndex, pageSize int) ([]model.Sharing, int64, error) {
	s, cnt, err := db.GetSharings(pageIndex, pageSize)
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return makeJoined(s), cnt, nil
}

func GetSharingsByCreatorId(userId uint, pageIndex, pageSize int) ([]model.Sharing, int64, error) {
	s, cnt, err := db.GetSharingsByCreatorId(userId, pageIndex, pageSize)
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}
	return makeJoined(s), cnt, nil
}

func GetSharingUnwrapPath(sharing *model.Sharing, path string) (unwrapPath string, err error) {
	if len(sharing.Files) == 0 {
		return "", errors.New("cannot get actual path of an invalid sharing")
	}
	if len(sharing.Files) == 1 {
		return stdpath.Join(sharing.Files[0], path), nil
	}
	path = utils.FixAndCleanPath(path)[1:]
	if len(path) == 0 {
		return "", errors.New("cannot get actual path of a sharing root path")
	}
	mapPath := ""
	child, rest, _ := strings.Cut(path, "/")
	for _, c := range sharing.Files {
		if child == stdpath.Base(c) {
			mapPath = c
			break
		}
	}
	if mapPath == "" {
		return "", fmt.Errorf("failed find child [%s] of sharing [%s]", child, sharing.ID)
	}
	return stdpath.Join(mapPath, rest), nil
}

func CreateSharing(sharing *model.Sharing) (id string, err error) {
	sharing.CreatorId = sharing.Creator.ID
	sharing.FilesRaw, err = utils.Json.MarshalToString(utils.MustSliceConvert(sharing.Files, utils.FixAndCleanPath))
	if err != nil {
		return "", errors.WithStack(err)
	}
	
	// Try to create platform-specific share for supported drivers
	if len(sharing.Files) > 0 {
		// Get the first file to check storage type
		filePath := sharing.Files[0]
		storage, actualPath, err := GetStorageAndActualPath(filePath)
		if err == nil {
			// Check if storage supports share creation (123_open driver)
			if storage.GetStorage().Driver == "123 Open" {
				// Try to create platform share
				if otherDriver, ok := storage.(driver.Other); ok {
					// Get the file object
					obj, err := Get(context.Background(), storage, actualPath)
					if err == nil {
						// Calculate expiration time
						var expireTime int64
						if sharing.Expires != nil && !sharing.Expires.IsZero() {
							expireTime = sharing.Expires.Unix()
						}
						
						// Create platform share
						result, err := otherDriver.Other(context.Background(), model.OtherArgs{
							Obj:    obj,
							Method: "create_share",
							Data: map[string]interface{}{
								"password":    sharing.Pwd,
								"expire_time": expireTime,
							},
						})
						
						if err == nil {
							// Extract share URL from result
							if resultMap, ok := result.(map[string]interface{}); ok {
								if shareURL, ok := resultMap["shareURL"].(string); ok {
									sharing.ExternalShareURL = shareURL
									log.Infof("Created 123pan official share: %s", shareURL)
								}
							}
						} else {
							log.Warnf("Failed to create platform share: %v", err)
						}
					}
				}
			}
		}
	}
	
	return db.CreateSharing(sharing.SharingDB)
}

func UpdateSharing(sharing *model.Sharing, skipMarshal ...bool) (err error) {
	if !utils.IsBool(skipMarshal...) {
		sharing.CreatorId = sharing.Creator.ID
		sharing.FilesRaw, err = utils.Json.MarshalToString(utils.MustSliceConvert(sharing.Files, utils.FixAndCleanPath))
		if err != nil {
			return errors.WithStack(err)
		}
	}
	sharingCache.Del(sharing.ID)
	return db.UpdateSharing(sharing.SharingDB)
}

func DeleteSharing(sid string) error {
	sharingCache.Del(sid)
	return db.DeleteSharingById(sid)
}

func DeleteSharingsByCreatorId(creatorId uint) error {
	return db.DeleteSharingsByCreatorId(creatorId)
}
