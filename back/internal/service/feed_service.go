package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/util"
	mysql2 "simple_tiktok/internal/repository/mysql"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type FeedService struct {
	videoRepo   *mysql2.VideoRepo
	userRepo    *mysql2.UserRepo
	redisClient *redis.Client
	hotMQ       *amqp.Channel
}

func NewFeedService(
	videoRepo *mysql2.VideoRepo,
	userRepo *mysql2.UserRepo,
	redisClient *redis.Client,
	hotMQ *amqp.Channel,
) *FeedService {
	return &FeedService{
		videoRepo:   videoRepo,
		userRepo:    userRepo,
		redisClient: redisClient,
		hotMQ:       hotMQ,
	}
}

func (s *FeedService) GetFeedVideos(limit uint64, lastScore float64, key string, userID uint64) ([]res.VideoInfoRes, float64, error) {
	ids, err := s.GetFeedVideoIds(limit, lastScore, key)
	if err != nil {
		return nil, 0.0, err
	}
	if len(ids) == 0 {
		return []res.VideoInfoRes{}, 0.0, nil
	}
	videoInfoList, err := s.getVideoInfoByIDs(ids)
	if err != nil {
		return nil, 0.0, err
	}
	videoInfoList = s.reorderExistingVideoInfos(videoInfoList, ids)
	s.fillVideoAuthorAvatar(videoInfoList)
	if err = s.fillVideoLikeStatus(videoInfoList, userID); err != nil {
		return nil, 0.0, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userID); err != nil {
		return nil, 0.0, err
	}
	nextScore := float64(videoInfoList[len(videoInfoList)-1].CreatedAt.UnixMicro())
	return videoInfoList, nextScore, nil
}

func (s *FeedService) GetFeedHotVideos(limit uint64, offset uint64, interval int, userID uint64) ([]res.VideoInfoRes, uint64, bool, error) {
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

	videoInfoList, err := s.getVideoInfoByIDs(ids)
	if err != nil {
		return nil, offset, false, err
	}
	videoInfoList = s.reorderExistingVideoInfos(videoInfoList, ids)
	if len(videoInfoList) == 0 {
		return []res.VideoInfoRes{}, offset + consumed, hasMore, nil
	}

	s.fillVideoAuthorAvatar(videoInfoList)
	if err = s.fillVideoLikeStatus(videoInfoList, userID); err != nil {
		return nil, offset, false, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userID); err != nil {
		return nil, offset, false, err
	}
	return videoInfoList, offset + consumed, hasMore, nil
}

func (s *FeedService) GetFollowFeedVideos(limit uint64, lastScore float64, userID uint64) ([]res.VideoInfoRes, float64, error) {
	followingIDs, err := s.getFollowingUserIDs(userID)
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
	if err = s.fillVideoLikeStatus(videoInfoList, userID); err != nil {
		return nil, 0, err
	}
	if err = s.fillVideoFollowStatus(videoInfoList, userID); err != nil {
		return nil, 0, err
	}
	nextScore := float64(videoInfoList[len(videoInfoList)-1].CreatedAt.UnixMicro())
	return videoInfoList, nextScore, nil
}

func (s *FeedService) GetFeedVideoIds(limit uint64, lastScore float64, key string) ([]uint64, error) {
	member := &redis.ZRangeBy{Min: "-inf", Max: "+inf", Offset: 0, Count: int64(limit)}
	if lastScore > 0 {
		member.Max = "(" + strconv.FormatFloat(lastScore, 'f', -1, 64)
	}
	idsStr, err := s.redisClient.ZRevRangeByScore(context.Background(), key, member).Result()
	if err != nil {
		return nil, err
	}
	videoIDs := make([]uint64, len(idsStr))
	for i, v := range idsStr {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, err
		}
		videoIDs[i] = id
	}
	return videoIDs, nil
}

func (s *FeedService) GetHotVideoIDsByWindow(limit uint64, offset uint64, interval int) ([]uint64, uint64, bool, error) {
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
		if err = s.redisClient.ZUnionStore(ctx, mergeKey, &redis.ZStore{Keys: keys, Aggregate: "SUM"}).Err(); err != nil {
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

func (s *FeedService) IncrementHotScoreByMinute(videoID uint64, delta float64, minute time.Time) error {
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

func (s *FeedService) AddVideoToFeed(videoID uint64, createdAt time.Time) error {
	return s.redisClient.ZAdd(context.Background(), constants.FeedVideoKey, redis.Z{
		Score:  float64(createdAt.UnixMicro()),
		Member: videoID,
	}).Err()
}

func (s *FeedService) RemoveVideoFromFeed(videoID uint64) error {
	return s.redisClient.ZRem(context.Background(), constants.FeedVideoKey, videoID).Err()
}

func (s *FeedService) EnsureHotVideoMember(videoID uint64, minute time.Time) error {
	key := s.getHotMinuteKey(minute.UTC().Truncate(time.Minute))
	ctx := context.Background()
	pipe := s.redisClient.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: 0, Member: strconv.FormatUint(videoID, 10)})
	pipe.Expire(ctx, key, 70*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *FeedService) RemoveVideoFromHotMinuteBuckets(videoID uint64, interval int) error {
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

func (s *FeedService) InvalidateVideoInfoCache(videoID uint64) error {
	cacheKey := fmt.Sprintf(constants.VideoInfoCacheKey, videoID)
	return s.redisClient.Del(context.Background(), cacheKey).Err()
}

func (s *FeedService) PublishVideoHotEvent(videoID uint64, delta float64) error {
	if s.hotMQ == nil || delta == 0 {
		return nil
	}
	data, err := json.Marshal(event.VideoHotEvent{
		VideoId:     videoID,
		ScoreDelta:  delta,
		MinuteStamp: time.Now().UTC().Truncate(time.Minute).Unix(),
	})
	if err != nil {
		return err
	}
	msg := amqp.Publishing{ContentType: "application/json", Body: data}
	return s.hotMQ.Publish(event.VideoHotExchange, event.VideoHotRoutingKey, false, false, msg)
}

func (s *FeedService) MustInvalidateVideoInfoCache(videoID uint64) {
	if err := s.InvalidateVideoInfoCache(videoID); err != nil {
		log.Println(err)
	}
}

func (s *FeedService) fillVideoAuthorAvatar(videoInfoList []res.VideoInfoRes) {
	cache := make(map[uint64]res.UserInfoRes)
	for i := range videoInfoList {
		authorID := videoInfoList[i].AuthorID
		if authorID == 0 {
			continue
		}
		if profile, ok := cache[authorID]; ok {
			videoInfoList[i].AuthorName = profile.Nickname
			videoInfoList[i].AuthorAvatar = profile.AvatarURL
			continue
		}
		user, err := s.userRepo.GetUserByID(authorID)
		if err != nil || user == nil {
			continue
		}
		profile := res.UserInfoRes{Nickname: user.NickName, AvatarURL: util.EnsureHTTPPath(user.AvatarURL)}
		cache[authorID] = profile
		if profile.Nickname != "" {
			videoInfoList[i].AuthorName = profile.Nickname
		}
		videoInfoList[i].AuthorAvatar = profile.AvatarURL
	}
}

func (s *FeedService) fillVideoLikeStatus(videoInfoList []res.VideoInfoRes, userID uint64) error {
	for i := range videoInfoList {
		likeKey := fmt.Sprintf(constants.LikeVideo, videoInfoList[i].Id)
		isLiked, err := s.redisClient.SIsMember(context.Background(), likeKey, userID).Result()
		if err != nil {
			return err
		}
		videoInfoList[i].IsLiked = isLiked
	}
	return nil
}

func (s *FeedService) fillVideoFollowStatus(videoInfoList []res.VideoInfoRes, userID uint64) error {
	followKey := fmt.Sprintf(constants.FollowKey, userID)
	for i := range videoInfoList {
		if videoInfoList[i].AuthorID == 0 || videoInfoList[i].AuthorID == userID {
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

func (s *FeedService) getFollowingUserIDs(userID uint64) ([]uint64, error) {
	key := fmt.Sprintf(constants.FollowKey, userID)
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

func (s *FeedService) getHotMinuteKey(t time.Time) string {
	return fmt.Sprintf("%s:%s", constants.HotFeedVideoMinutePrefix, t.Format("200601021504"))
}

func (s *FeedService) getHotMergeKey(t time.Time, interval int) string {
	return fmt.Sprintf("%s:%d:%s", constants.HotFeedVideoMergePrefix, interval, t.Format("200601021504"))
}

func (s *FeedService) reorderExistingVideoInfos(videoInfoList []res.VideoInfoRes, ids []uint64) []res.VideoInfoRes {
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

func (s *FeedService) getVideoInfoByIDs(ids []uint64) ([]res.VideoInfoRes, error) {
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
