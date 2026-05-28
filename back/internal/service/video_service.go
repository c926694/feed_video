package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
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

func NewVideoService(videoRepo *mysql2.VideoRepo, userRepo *mysql2.UserRepo, redisClient *redis.Client,
	videoMQ *amqp.Channel, commentRepo *mysql2.CommentRepo) *VideoService {
	return &VideoService{
		videoRepo:   videoRepo,
		userRepo:    userRepo,
		redisClient: redisClient,
		videoMQ:     videoMQ,
		commentRepo: commentRepo,
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

	err = s.AddZSet(constants.FeedVideoKey, float64(video.CreatedAt.UnixMicro()), video.ID)
	if err != nil {
		return res.VideoRes{}, err
	}
	if err = s.EnsureHotVideoMember(video.ID, time.Now()); err != nil {
		return res.VideoRes{}, err
	}
	return res.VideoRes{
		Id:  video.ID,
		Url: util.EnsureHTTPPath(video.PlayURL),
	}, nil
}

func (s *VideoService) AddZSet(key string, score float64, member uint64) error {
	value := redis.Z{
		Score:  score,
		Member: member,
	}
	_, err := s.redisClient.ZAdd(context.Background(), key, value).Result()
	if err != nil {
		return err
	}
	return nil
}

func (s *VideoService) RemZSet(key string, member uint64) error {
	_, err := s.redisClient.ZRem(context.Background(), key, member).Result()
	if err != nil {
		return err
	}
	return nil
}

func (s *VideoService) GetFeedVideos(limit uint64, lastScore float64, key string, userId uint64) ([]res.VideoInfoRes, float64, error) {
	ids, err := s.GetFeedVideoIds(limit, lastScore, key)
	if err != nil {
		return nil, 0.0, err
	}
	if len(ids) == 0 {
		return []res.VideoInfoRes{}, 0.0, nil
	}
	videoInfoList, err := s.getVideoInfoByIds(ids)
	if err != nil {
		return nil, 0.0, err
	}
	videoInfoList, err = s.getFinalVideoInfoResList(videoInfoList, ids)
	if err != nil {
		return nil, 0.0, err
	}
	s.fillVideoAuthorAvatar(videoInfoList)
	if err = s.fillVideoLikeStatus(videoInfoList, userId); err != nil {
		return nil, 0.0, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userId); err != nil {
		return nil, 0.0, err
	}
	nextScore := float64(videoInfoList[len(videoInfoList)-1].CreatedAt.UnixMicro())
	return videoInfoList, nextScore, nil
}

func (s *VideoService) getFinalVideoInfoResList(videoInfoList []res.VideoInfoRes, ids []uint64) ([]res.VideoInfoRes, error) {
	videoMap := make(map[uint64]res.VideoInfoRes)
	for _, videoInfo := range videoInfoList {
		videoMap[videoInfo.Id] = videoInfo
	}
	result := make([]res.VideoInfoRes, len(ids))
	for i, id := range ids {
		result[i] = videoMap[id]
	}
	return result, nil
}

func (s *VideoService) GetFeedHotVideos(limit uint64, offset uint64, interval int, userId uint64) ([]res.VideoInfoRes, uint64, bool, error) {
	if limit == 0 {
		limit = 5
	}
	if interval <= 0 {
		interval = 60
	}
	if interval > 1440 {
		interval = 1440
	}

	ids, consumed, hasMore, err := s.GetHotVideoIDsByWindow(limit, offset, interval)
	if err != nil {
		return nil, offset, false, err
	}
	if len(ids) == 0 {
		return []res.VideoInfoRes{}, offset, false, nil
	}

	videoInfoList, err := s.getVideoInfoByIds(ids)
	if err != nil {
		return nil, offset, false, err
	}
	videoInfoList = s.reorderExistingVideoInfos(videoInfoList, ids)
	if len(videoInfoList) == 0 {
		return []res.VideoInfoRes{}, offset + consumed, hasMore, nil
	}

	s.fillVideoAuthorAvatar(videoInfoList)
	if err = s.fillVideoLikeStatus(videoInfoList, userId); err != nil {
		return nil, offset, false, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userId); err != nil {
		return nil, offset, false, err
	}
	return videoInfoList, offset + consumed, hasMore, nil
}

func (s *VideoService) GetFollowFeedVideos(limit uint64, lastScore float64, userId uint64) ([]res.VideoInfoRes, float64, error) {
	followingIDs, err := s.getFollowingUserIDs(userId)
	if err != nil {
		return nil, 0, err
	}
	if len(followingIDs) == 0 {
		return []res.VideoInfoRes{}, 0, nil
	}

	var lastCreatedAt *time.Time
	if lastScore > 0 && lastScore < float64(math.MaxInt64) {
		t := time.UnixMicro(int64(lastScore))
		lastCreatedAt = &t
	}

	videoList, err := s.videoRepo.GetFollowFeedVideosByAuthors(followingIDs, limit, lastCreatedAt)
	if err != nil {
		return nil, 0, err
	}
	if len(videoList) == 0 {
		return []res.VideoInfoRes{}, 0, nil
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
		return nil, 0, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userId); err != nil {
		return nil, 0, err
	}
	nextScore := float64(videoInfoList[len(videoInfoList)-1].CreatedAt.UnixMicro())
	return videoInfoList, nextScore, nil
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

func (s *VideoService) getFollowingUserIDs(userId uint64) ([]uint64, error) {
	key := fmt.Sprintf(constants.FollowKey, userId)
	members, err := s.redisClient.SMembers(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	result := make([]uint64, 0, len(members))
	for _, member := range members {
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, id)
	}
	return result, nil
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

func (s *VideoService) HotScore(likeCount, commentCount int64, videoId uint64) float64 {
	return float64(likeCount*2+commentCount) + float64(videoId)/1e10
}

func (s *VideoService) GetFeedVideoIds(limit uint64, lastScore float64, key string) ([]uint64, error) {
	member := &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	}
	if lastScore > 0 {
		member.Max = "(" + strconv.FormatFloat(lastScore, 'f', -1, 64)
	}
	//log.Println("max===", member.Max)
	idsStr, err := s.redisClient.ZRevRangeByScore(context.Background(), key, member).Result()
	if err != nil {
		return nil, err
	}
	videoIds := make([]uint64, len(idsStr))
	for i, v := range idsStr {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, err
		}
		videoIds[i] = id
	}
	return videoIds, nil
}

func (s *VideoService) GetHotVideoIDsByWindow(limit uint64, offset uint64, interval int) ([]uint64, uint64, bool, error) {
	keys := make([]string, 0, interval)
	now := time.Now().UTC().Truncate(time.Minute)
	for i := 0; i < interval; i++ {
		keys = append(keys, s.getHotMinuteKey(now.Add(-time.Duration(i)*time.Minute)))
	}

	mergeKey := s.getHotMergeKey(now, interval)
	ctx := context.Background()
	if ok, err := s.redisClient.Exists(ctx, mergeKey).Result(); err != nil {
		return nil, 0, false, err
	} else if ok == 0 {
		if err = s.redisClient.ZUnionStore(ctx, mergeKey, &redis.ZStore{
			Keys:      keys,
			Aggregate: "SUM",
		}).Err(); err != nil {
			return nil, 0, false, err
		}
		if err = s.redisClient.Expire(ctx, mergeKey, 2*time.Minute).Err(); err != nil {
			return nil, 0, false, err
		}
	}

	start := int64(offset)
	stop := start + int64(limit)
	idsStr, err := s.redisClient.ZRevRange(ctx, mergeKey, start, stop).Result()
	if err != nil {
		return nil, 0, false, err
	}
	if len(idsStr) == 0 {
		return []uint64{}, 0, false, nil
	}

	hasMore := len(idsStr) > int(limit)
	if hasMore {
		idsStr = idsStr[:limit]
	}
	ids := make([]uint64, 0, len(idsStr))
	for _, raw := range idsStr {
		id, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, uint64(len(idsStr)), hasMore, nil
}

func (s *VideoService) IncrementHotScoreByMinute(videoID uint64, delta float64, minute time.Time) error {
	if delta == 0 {
		return nil
	}
	key := s.getHotMinuteKey(minute.UTC().Truncate(time.Minute))
	ctx := context.Background()
	pipe := s.redisClient.TxPipeline()
	pipe.ZIncrBy(ctx, key, delta, strconv.FormatUint(videoID, 10))
	pipe.Expire(ctx, key, 70*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *VideoService) EnsureHotVideoMember(videoID uint64, minute time.Time) error {
	key := s.getHotMinuteKey(minute.UTC().Truncate(time.Minute))
	ctx := context.Background()
	pipe := s.redisClient.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  0,
		Member: strconv.FormatUint(videoID, 10),
	})
	pipe.Expire(ctx, key, 70*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *VideoService) RemoveVideoFromHotMinuteBuckets(videoID uint64, interval int) error {
	if interval <= 0 {
		return nil
	}
	now := time.Now().UTC().Truncate(time.Minute)
	ctx := context.Background()
	pipe := s.redisClient.TxPipeline()
	for i := 0; i < interval; i++ {
		key := s.getHotMinuteKey(now.Add(-time.Duration(i) * time.Minute))
		pipe.ZRem(ctx, key, strconv.FormatUint(videoID, 10))
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *VideoService) getHotMinuteKey(t time.Time) string {
	return fmt.Sprintf("%s:%s", constants.HotFeedVideoMinutePrefix, t.Format("200601021504"))
}

func (s *VideoService) getHotMergeKey(t time.Time, interval int) string {
	return fmt.Sprintf("%s:%d:%s", constants.HotFeedVideoMergePrefix, interval, t.Format("200601021504"))
}

func (s *VideoService) reorderExistingVideoInfos(videoInfoList []res.VideoInfoRes, ids []uint64) []res.VideoInfoRes {
	videoMap := make(map[uint64]res.VideoInfoRes, len(videoInfoList))
	for _, videoInfo := range videoInfoList {
		videoMap[videoInfo.Id] = videoInfo
	}
	result := make([]res.VideoInfoRes, 0, len(ids))
	for _, id := range ids {
		videoInfo, ok := videoMap[id]
		if !ok {
			continue
		}
		result = append(result, videoInfo)
	}
	return result
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

func (s *VideoService) getVideoInfoByIds(ids []uint64) ([]res.VideoInfoRes, error) {
	videoList, err := s.videoRepo.GetFeedVideos(ids)
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
	return videoInfoList, nil
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

	if err := s.RemZSet(constants.FeedVideoKey, id); err != nil {
		return err
	}
	if err := s.RemoveVideoFromHotMinuteBuckets(id, 1440); err != nil {
		return err
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

func (s *VideoService) UpdateZSet(key string, score float64, id uint64) error {
	member := redis.Z{
		Score:  score,
		Member: id,
	}
	return s.redisClient.ZAdd(context.Background(), key, member).Err()
}
