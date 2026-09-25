package repo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	feedVideoKey    = "feed:video"
	hotMinutePrefix = "feed:hot:video:1m"
	hotMergePrefix  = "feed:hot:video:merge"
	hotMinuteTTL    = 70 * time.Minute
	hotMergeTTL     = 2 * time.Minute
)

// Repo Feed 索引与热度分钟桶的读写
type Repo struct {
	redisClient *redis.Client
}

func New(redisClient *redis.Client) *Repo {
	return &Repo{redisClient: redisClient}
}

// AddToFeed 把视频按发布时间加进 Feed 索引
func (r *Repo) AddToFeed(ctx context.Context, videoID uint64, createdAt time.Time) error {
	return r.redisClient.ZAdd(ctx, feedVideoKey, redis.Z{
		Score:  float64(createdAt.UnixMicro()),
		Member: videoID,
	}).Err()
}

// RemoveFromFeed 把视频移出 Feed 索引
func (r *Repo) RemoveFromFeed(ctx context.Context, videoID uint64) error {
	return r.redisClient.ZRem(ctx, feedVideoKey, videoID).Err()
}

// FeedIDs 按发布时间倒序取一页视频 ID
func (r *Repo) FeedIDs(ctx context.Context, limit uint64, lastScore float64) ([]uint64, error) {
	member := &redis.ZRangeBy{Min: "-inf", Max: "+inf", Offset: 0, Count: int64(limit)}
	if lastScore > 0 {
		member.Max = "(" + strconv.FormatFloat(lastScore, 'f', -1, 64)
	}
	rawIDs, err := r.redisClient.ZRevRangeByScore(ctx, feedVideoKey, member).Result()
	if err != nil {
		return nil, err
	}
	return parseIDs(rawIDs)
}

// EnsureHotMember 保证视频出现在当前分钟的热度桶里
func (r *Repo) EnsureHotMember(ctx context.Context, videoID uint64, minute time.Time) error {
	key := hotMinuteKey(minute.UTC().Truncate(time.Minute))
	pipe := r.redisClient.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: 0, Member: strconv.FormatUint(videoID, 10)})
	pipe.Expire(ctx, key, hotMinuteTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// IncreaseHotScore 给视频的热度加分
func (r *Repo) IncreaseHotScore(ctx context.Context, videoID uint64, delta float64, minute time.Time) error {
	if delta == 0 {
		return nil
	}
	key := hotMinuteKey(minute.UTC().Truncate(time.Minute))
	pipe := r.redisClient.TxPipeline()
	pipe.ZIncrBy(ctx, key, delta, strconv.FormatUint(videoID, 10))
	pipe.Expire(ctx, key, hotMinuteTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// RemoveFromHotBuckets 把视频从最近若干分钟的桶里移除
func (r *Repo) RemoveFromHotBuckets(ctx context.Context, videoID uint64, interval int) error {
	if interval <= 0 {
		return nil
	}
	now := time.Now().UTC().Truncate(time.Minute)
	member := strconv.FormatUint(videoID, 10)
	pipe := r.redisClient.TxPipeline()
	for i := 0; i < interval; i++ {
		pipe.ZRem(ctx, hotMinuteKey(now.Add(-time.Duration(i)*time.Minute)), member)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// HotIDs 把最近 interval 分钟的桶合并后按分数倒序取一页
func (r *Repo) HotIDs(ctx context.Context, limit uint64, offset uint64, interval int) ([]uint64, uint64, bool, error) {
	now := time.Now().UTC().Truncate(time.Minute)
	keys := make([]string, 0, interval)
	for i := 0; i < interval; i++ {
		keys = append(keys, hotMinuteKey(now.Add(-time.Duration(i)*time.Minute)))
	}

	mergeKey := hotMergeKey(now, interval)
	exists, err := r.redisClient.Exists(ctx, mergeKey).Result()
	if err != nil {
		return nil, 0, false, err
	}
	if exists == 0 {
		if err = r.redisClient.ZUnionStore(ctx, mergeKey, &redis.ZStore{Keys: keys, Aggregate: "SUM"}).Err(); err != nil {
			return nil, 0, false, err
		}
		if err = r.redisClient.Expire(ctx, mergeKey, hotMergeTTL).Err(); err != nil {
			return nil, 0, false, err
		}
	}

	start := int64(offset)
	stop := start + int64(limit)
	rawIDs, err := r.redisClient.ZRevRange(ctx, mergeKey, start, stop).Result()
	if err != nil {
		return nil, 0, false, err
	}
	if len(rawIDs) == 0 {
		return []uint64{}, 0, false, nil
	}
	hasMore := len(rawIDs) > int(limit)
	if hasMore {
		rawIDs = rawIDs[:limit]
	}
	ids, err := parseIDs(rawIDs)
	if err != nil {
		return nil, 0, false, err
	}
	return ids, uint64(len(rawIDs)), hasMore, nil
}

func parseIDs(rawIDs []string) ([]uint64, error) {
	ids := make([]uint64, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func hotMinuteKey(minute time.Time) string {
	return fmt.Sprintf("%s:%s", hotMinutePrefix, minute.Format("200601021504"))
}

func hotMergeKey(minute time.Time, interval int) string {
	return fmt.Sprintf("%s:%d:%s", hotMergePrefix, interval, minute.Format("200601021504"))
}
