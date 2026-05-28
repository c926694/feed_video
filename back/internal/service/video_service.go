package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/model"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/upload"
	"simple_tiktok/internal/pkg/util"
	mysql2 "simple_tiktok/internal/repository/mysql"
	"strconv"
	"strings"
	"time"

	//"github.com/redis/go-redis/v9"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type VideoService struct {
	videoRepo   *mysql2.VideoRepo
	userRepo    *mysql2.UserRepo
	redisClient *redis.Client
	videoMQ     *amqp.Channel
	commentRepo *mysql2.CommentRepo
	feedService *FeedService
}

type videoInfoCacheEnvelope struct {
	Data     *res.VideoInfoRes `json:"data,omitempty"`
	Empty    bool              `json:"empty"`
	ExpireAt int64             `json:"expire_at"`
}

const (
	videoInfoLogicalTTL     = 5 * time.Minute
	videoInfoNullLogicalTTL = 2 * time.Minute
	videoInfoPhysicalTTL    = 24 * time.Hour
	videoInfoRebuildLockTTL = 10 * time.Second
	videoInfoMissRetryTimes = 8
	videoInfoMissRetrySleep = 30 * time.Millisecond
)

var unlockVideoInfoLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)

func NewVideoService(
	videoRepo *mysql2.VideoRepo,
	userRepo *mysql2.UserRepo,
	redisClient *redis.Client,
	videoMQ *amqp.Channel,
	commentRepo *mysql2.CommentRepo,
	feedService *FeedService,
) *VideoService {
	return &VideoService{
		videoRepo:   videoRepo,
		userRepo:    userRepo,
		redisClient: redisClient,
		videoMQ:     videoMQ,
		commentRepo: commentRepo,
		feedService: feedService,
	}
}

func (s *VideoService) CreateVideo(req req.UploadVideoReq, userId uint64, nickName string) (res.VideoRes, error) {
	cover := req.Cover
	play := req.Play
	title := req.Title
	description := req.Description

	authorName := strings.TrimSpace(nickName)
	if s.userRepo != nil {
		user, userErr := s.userRepo.GetUserByID(userId)
		if userErr == nil && user != nil {
			currentNickName := strings.TrimSpace(user.NickName)
			if currentNickName != "" {
				authorName = currentNickName
			}
		}
	}

	coverPath, err := upload.UploadFile(cover, upload.Cover)
	if err != nil {
		return res.VideoRes{}, err
	}

	playPath, err := upload.UploadFile(play, upload.Video)
	if err != nil {
		err2 := upload.Delete(upload.Cover, coverPath)
		if err2 != nil {
			return res.VideoRes{}, err2
		}
		return res.VideoRes{}, err
	}

	video := model.Video{
		Title:       title,
		Description: description,
		AuthorID:    userId,
		PlayURL:     playPath,
		CoverURL:    coverPath,
		AuthorName:  authorName,
	}
	log.Println("nickName:", authorName)

	err = s.videoRepo.CreateVideo(&video)
	if err != nil {
		err2 := upload.Delete(upload.Video, playPath)
		if err2 != nil {
			return res.VideoRes{}, err2
		}
		return res.VideoRes{}, err
	}

	if s.feedService != nil {
		err = s.feedService.AddVideoToFeed(video.ID, video.CreatedAt)
		if err != nil {
			return res.VideoRes{}, err
		}
		if err = s.feedService.EnsureHotVideoMember(video.ID, time.Now()); err != nil {
			return res.VideoRes{}, err
		}
	}
	return res.VideoRes{
		Id:  video.ID,
		Url: util.EnsureHTTPPath(video.PlayURL),
	}, nil
}

func (s *VideoService) GetMyVideos(userId uint64, limit uint64) ([]res.VideoInfoRes, error) {
	videoList, err := s.videoRepo.ListByAuthorID(userId, limit)
	if err != nil {
		return nil, err
	}
	videoInfoList := make([]res.VideoInfoRes, len(videoList))
	for i, v := range videoList {
		videoInfoList[i] = res.VideoInfoRes{
			Id:           v.ID,
			AuthorID:     v.AuthorID,
			AuthorName:   v.AuthorName,
			Title:        v.Title,
			Description:  v.Description,
			CoverURL:     util.EnsureHTTPPath(v.CoverURL),
			PlayURL:      util.EnsureHTTPPath(v.PlayURL),
			CreatedAt:    v.CreatedAt,
			LikeCount:    v.LikeCount,
			CommentCount: v.CommentCount,
		}
	}
	s.fillVideoAuthorAvatar(videoInfoList)
	if err = s.fillVideoLikeStatus(videoInfoList, userId); err != nil {
		return nil, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userId); err != nil {
		return nil, err
	}
	return videoInfoList, nil
}

func (s *VideoService) fillVideoAuthorAvatar(videoInfoList []res.VideoInfoRes) {
	if s.userRepo == nil {
		return
	}
	type authorProfile struct {
		name   string
		avatar string
	}
	cache := make(map[uint64]authorProfile)
	for i := range videoInfoList {
		authorID := videoInfoList[i].AuthorID
		if authorID == 0 {
			continue
		}
		if profile, ok := cache[authorID]; ok {
			if profile.name != "" {
				videoInfoList[i].AuthorName = profile.name
			}
			videoInfoList[i].AuthorAvatar = profile.avatar
			continue
		}
		user, err := s.userRepo.GetUserByID(authorID)
		if err != nil || user == nil {
			continue
		}
		profile := authorProfile{
			name:   user.NickName,
			avatar: util.EnsureHTTPPath(user.AvatarURL),
		}
		cache[authorID] = profile
		if profile.name != "" {
			videoInfoList[i].AuthorName = profile.name
		}
		videoInfoList[i].AuthorAvatar = profile.avatar
	}
}

func (s *VideoService) fillVideoLikeStatus(videoInfoList []res.VideoInfoRes, userId uint64) error {
	for i := range videoInfoList {
		likeKey := fmt.Sprintf(constants.LikeVideo, videoInfoList[i].Id)
		isLiked, err := s.redisClient.SIsMember(context.Background(), likeKey, userId).Result()
		if err != nil {
			return err
		}
		videoInfoList[i].IsLiked = isLiked
	}
	return nil
}

func (s *VideoService) fillVideoFollowStatus(videoInfoList []res.VideoInfoRes, userId uint64) error {
	followKey := fmt.Sprintf(constants.FollowKey, userId)
	for i := range videoInfoList {
		if videoInfoList[i].AuthorID == 0 || videoInfoList[i].AuthorID == userId {
			videoInfoList[i].IsFollow = false
			continue
		}
		isFollow, err := s.redisClient.SIsMember(context.Background(), followKey, videoInfoList[i].AuthorID).Result()
		if err != nil {
			return err
		}
		videoInfoList[i].IsFollow = isFollow
	}
	return nil
}

func (s *VideoService) GetVideoInfo(id uint64, userId uint64) (res.VideoInfoRes, error) {
	videoInfo, exists, err := s.getVideoInfoBaseWithCache(id)
	if err != nil {
		return res.VideoInfoRes{}, err
	}
	if !exists {
		return res.VideoInfoRes{}, gorm.ErrRecordNotFound
	}
	videoInfoList := []res.VideoInfoRes{videoInfo}
	if err = s.fillVideoLikeStatus(videoInfoList, userId); err != nil {
		return res.VideoInfoRes{}, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userId); err != nil {
		return res.VideoInfoRes{}, err
	}
	return videoInfoList[0], nil
}

func (s *VideoService) getVideoInfoBaseWithCache(videoID uint64) (res.VideoInfoRes, bool, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf(constants.VideoInfoCacheKey, videoID)
	raw, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		envelope, parseErr := s.parseVideoInfoCacheEnvelope(raw)
		if parseErr == nil {
			now := time.Now().Unix()
			if envelope.Empty {
				if envelope.ExpireAt <= now {
					s.tryRefreshVideoInfoCacheAsync(videoID)
				}
				return res.VideoInfoRes{}, false, nil
			}
			if envelope.Data != nil {
				if envelope.ExpireAt <= now {
					s.tryRefreshVideoInfoCacheAsync(videoID)
				}
				return *envelope.Data, true, nil
			}
		}
		_ = s.redisClient.Del(ctx, cacheKey).Err()
	} else if err != redis.Nil {
		return res.VideoInfoRes{}, false, err
	}

	return s.rebuildVideoInfoCacheOnMiss(videoID)
}

func (s *VideoService) rebuildVideoInfoCacheOnMiss(videoID uint64) (res.VideoInfoRes, bool, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf(constants.VideoInfoCacheKey, videoID)
	lockKey := fmt.Sprintf(constants.VideoInfoLockKey, videoID)
	for i := 0; i < videoInfoMissRetryTimes; i++ {
		lockValue := strconv.FormatInt(time.Now().UnixNano(), 10)
		locked, err := s.redisClient.SetNX(ctx, lockKey, lockValue, videoInfoRebuildLockTTL).Result()
		if err != nil {
			return res.VideoInfoRes{}, false, err
		}
		if locked {
			defer s.releaseVideoInfoLock(lockKey, lockValue)

			if raw, getErr := s.redisClient.Get(ctx, cacheKey).Result(); getErr == nil {
				envelope, parseErr := s.parseVideoInfoCacheEnvelope(raw)
				if parseErr == nil {
					if envelope.Empty {
						return res.VideoInfoRes{}, false, nil
					}
					if envelope.Data != nil {
						return *envelope.Data, true, nil
					}
				}
			}

			videoInfo, exists, loadErr := s.loadVideoInfoFromDBAndWriteCache(videoID)
			return videoInfo, exists, loadErr
		}

		time.Sleep(videoInfoMissRetrySleep)
		raw, getErr := s.redisClient.Get(ctx, cacheKey).Result()
		if getErr != nil {
			if getErr == redis.Nil {
				continue
			}
			return res.VideoInfoRes{}, false, getErr
		}
		envelope, parseErr := s.parseVideoInfoCacheEnvelope(raw)
		if parseErr != nil {
			continue
		}
		if envelope.Empty {
			return res.VideoInfoRes{}, false, nil
		}
		if envelope.Data != nil {
			return *envelope.Data, true, nil
		}
	}
	return s.loadVideoInfoFromDBAndWriteCache(videoID)
}

func (s *VideoService) tryRefreshVideoInfoCacheAsync(videoID uint64) {
	ctx := context.Background()
	lockKey := fmt.Sprintf(constants.VideoInfoLockKey, videoID)
	lockValue := strconv.FormatInt(time.Now().UnixNano(), 10)
	locked, err := s.redisClient.SetNX(ctx, lockKey, lockValue, videoInfoRebuildLockTTL).Result()
	if err != nil || !locked {
		return
	}
	go func() {
		defer s.releaseVideoInfoLock(lockKey, lockValue)
		_, _, _ = s.loadVideoInfoFromDBAndWriteCache(videoID)
	}()
}

func (s *VideoService) loadVideoInfoFromDBAndWriteCache(videoID uint64) (res.VideoInfoRes, bool, error) {
	videoInfo, err := s.loadVideoInfoBaseFromDB(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			setErr := s.setVideoInfoCacheEnvelope(videoID, videoInfoCacheEnvelope{
				Empty:    true,
				ExpireAt: time.Now().Add(videoInfoNullLogicalTTL).Unix(),
			})
			if setErr != nil {
				return res.VideoInfoRes{}, false, setErr
			}
			return res.VideoInfoRes{}, false, nil
		}
		return res.VideoInfoRes{}, false, err
	}
	setErr := s.setVideoInfoCacheEnvelope(videoID, videoInfoCacheEnvelope{
		Data:     &videoInfo,
		ExpireAt: time.Now().Add(videoInfoLogicalTTL).Unix(),
	})
	if setErr != nil {
		return res.VideoInfoRes{}, false, setErr
	}
	return videoInfo, true, nil
}

func (s *VideoService) loadVideoInfoBaseFromDB(videoID uint64) (res.VideoInfoRes, error) {
	video, err := s.videoRepo.GetVideoById(videoID)
	if err != nil {
		return res.VideoInfoRes{}, err
	}
	videoInfo := res.VideoInfoRes{
		Id:           video.ID,
		AuthorID:     video.AuthorID,
		AuthorName:   video.AuthorName,
		Title:        video.Title,
		Description:  video.Description,
		CoverURL:     util.EnsureHTTPPath(video.CoverURL),
		PlayURL:      util.EnsureHTTPPath(video.PlayURL),
		CommentCount: video.CommentCount,
		LikeCount:    video.LikeCount,
		CreatedAt:    video.CreatedAt,
	}
	videoInfoList := []res.VideoInfoRes{videoInfo}
	s.fillVideoAuthorAvatar(videoInfoList)
	return videoInfoList[0], nil
}

func (s *VideoService) setVideoInfoCacheEnvelope(videoID uint64, envelope videoInfoCacheEnvelope) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	cacheKey := fmt.Sprintf(constants.VideoInfoCacheKey, videoID)
	return s.redisClient.Set(context.Background(), cacheKey, data, videoInfoPhysicalTTL).Err()
}

func (s *VideoService) parseVideoInfoCacheEnvelope(raw string) (videoInfoCacheEnvelope, error) {
	var envelope videoInfoCacheEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return videoInfoCacheEnvelope{}, err
	}
	return envelope, nil
}

func (s *VideoService) releaseVideoInfoLock(lockKey string, lockValue string) {
	_ = unlockVideoInfoLockScript.Run(context.Background(), s.redisClient, []string{lockKey}, lockValue).Err()
}

func (s *VideoService) invalidateVideoInfoCache(videoID uint64) {
	cacheKey := fmt.Sprintf(constants.VideoInfoCacheKey, videoID)
	_ = s.redisClient.Del(context.Background(), cacheKey).Err()
}

func (s *VideoService) DeleteVideo(id uint64, userId uint64) error {
	tx := s.videoRepo.DB().Begin()
	video, err := s.videoRepo.GetVideoById(id)
	if err != nil {
		return err
	}
	if video.AuthorID != userId {
		_ = tx.Rollback()
		return errors.New("no permission to delete this video")
	}

	commentRepo := s.commentRepo.WithTx(tx)
	commentList, err := commentRepo.ListByVideoId(id)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	videoRepo := s.videoRepo.WithTx(tx)
	if err := videoRepo.DeleteVideoById(id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := commentRepo.DeleteByVideoId(id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}

	if s.feedService != nil {
		if err := s.feedService.RemoveVideoFromFeed(id); err != nil {
			return err
		}
		if err := s.feedService.RemoveVideoFromHotMinuteBuckets(id, 1440); err != nil {
			return err
		}
	}
	s.invalidateVideoInfoCache(id)

	likeKey := fmt.Sprintf(constants.LikeVideo, id)
	if err := s.redisClient.Del(context.Background(), likeKey).Err(); err != nil {
		return err
	}
	if len(commentList) > 0 {
		commentLikeKeys := make([]string, 0, len(commentList))
		for _, comment := range commentList {
			commentLikeKeys = append(commentLikeKeys, fmt.Sprintf(constants.LikeComment, comment.ID))
		}
		if err := s.redisClient.Del(context.Background(), commentLikeKeys...).Err(); err != nil {
			return err
		}
	}

	videoProducer := s.videoMQ
	msg, err := s.getDeleteVideoEvent(video)
	if err != nil {
		return err
	}
	err = videoProducer.Publish(event.DeleteVideoExchange,
		event.DeleteVideoRoutingKey, false, false, msg)
	if err != nil {
		return err
	}
	return nil
}

func (s *VideoService) getDeleteVideoEvent(video model.Video) (amqp.Publishing, error) {
	deleteVideoEvent := event.DeleteVideoEvent{
		PlayURL:  video.PlayURL,
		CoverURL: video.CoverURL,
	}
	data, err := json.Marshal(deleteVideoEvent)
	if err != nil {
		return amqp.Publishing{}, err
	}
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	}
	return msg, nil
}
