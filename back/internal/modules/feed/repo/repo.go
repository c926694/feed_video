package repo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	hotMinutePrefix = "feed:hot:video:1m"
	hotMergePrefix  = "feed:hot:video:merge"
	hotMarkPrefix   = "feed:hot:mark"

	// HotRetentionMinutes 热度数据的保留分钟数。分钟桶的存活时间与查询窗口的上限
	// 都由它决定，两者取同一个值，声明的窗口不会超过实际留得住的数据
	HotRetentionMinutes = 70

	hotMinuteTTL = HotRetentionMinutes * time.Minute
	hotMergeTTL  = 2 * time.Minute
)

// HotRetention 保留时长，供计分时判断事件是否已经超出窗口
const HotRetention = HotRetentionMinutes * time.Minute

// Repo Feed 索引与热度分钟桶的读写
type Repo struct {
	redisClient *redis.Client
}

func New(redisClient *redis.Client) *Repo {
	return &Repo{redisClient: redisClient}
}

// Pipeline 供读路径把多个模块的状态查询合并成一次往返
func (r *Repo) Pipeline() redis.Pipeliner {
	return r.redisClient.Pipeline()
}

// scoreOnceScript 窗口内同一个用户的同类互动只计一次分：
// 标记键写成功才给分钟桶加分，重复投递、反复取消再点赞都不会重复计分
var scoreOnceScript = redis.NewScript(`
local marked = redis.call("SET", KEYS[1], "1", "NX", "EX", ARGV[1])
if not marked then
    return 0
end
redis.call("ZINCRBY", KEYS[2], ARGV[2], ARGV[3])
redis.call("EXPIRE", KEYS[2], ARGV[4])
return 1
`)

// ScoreOnce 在窗口内给视频计一次分。occurredAt 决定计入哪一分钟的桶，
// 标记键的存活时间按"保留时长减去事件已经晚了多久"算，让标记与桶同时过期
func (r *Repo) ScoreOnce(ctx context.Context, scoreType string, videoID uint64, userID uint64, weight float64, occurredAt time.Time, now time.Time) (bool, error) {
	markTTL := int(HotRetention/time.Second) - int(now.Sub(occurredAt)/time.Second)
	if markTTL < 1 {
		markTTL = 1
	}
	result, err := scoreOnceScript.Run(ctx, r.redisClient,
		[]string{hotMarkKey(scoreType, videoID, userID), hotMinuteKey(occurredAt)},
		markTTL, weight, strconv.FormatUint(videoID, 10), int(hotMinuteTTL/time.Second),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// HotItem 榜单里的一个成员与它的分数
type HotItem struct {
	ID    uint64
	Score float64
}

// HotPage 把最近 interval 分钟的桶合并后按分数倒序取一页，分数一起返回供同分排序使用。
// 合并键只是这一分钟的缓存，不存在就重建
func (r *Repo) HotPage(ctx context.Context, limit uint64, offset uint64, interval int) ([]HotItem, uint64, bool, error) {
	now := time.Now().UTC().Truncate(time.Minute)
	mergeKey := hotMergeKey(now, interval)

	items, err := r.hotRange(ctx, mergeKey, limit, offset)
	if err != nil {
		return nil, 0, false, err
	}
	if len(items) == 0 {
		// 空结果可能是合并键不存在，也可能是这一页确实没有内容，用一次存在性判断区分
		exists, existsErr := r.redisClient.Exists(ctx, mergeKey).Result()
		if existsErr != nil {
			return nil, 0, false, existsErr
		}
		if exists == 0 {
			if err = r.mergeHotBuckets(ctx, mergeKey, now, interval); err != nil {
				return nil, 0, false, err
			}
			if items, err = r.hotRange(ctx, mergeKey, limit, offset); err != nil {
				return nil, 0, false, err
			}
		}
	}

	hasMore := len(items) > int(limit)
	if hasMore {
		items = items[:limit]
	}

	list := make([]HotItem, 0, len(items))
	for _, item := range items {
		member, ok := item.Member.(string)
		if !ok {
			return nil, 0, false, fmt.Errorf("热度桶里的成员不是字符串")
		}
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil {
			return nil, 0, false, parseErr
		}
		list = append(list, HotItem{ID: id, Score: item.Score})
	}
	return list, uint64(len(list)), hasMore, nil
}

// hotRange 在合并键上按分数倒序取一段，多取一条由调用方判断还有没有下一页
func (r *Repo) hotRange(ctx context.Context, mergeKey string, limit uint64, offset uint64) ([]redis.Z, error) {
	return r.redisClient.ZRevRangeWithScores(ctx, mergeKey, int64(offset), int64(offset+limit)).Result()
}

// mergeHotBuckets 把最近 interval 个分钟桶按分数求和写进合并键
func (r *Repo) mergeHotBuckets(ctx context.Context, mergeKey string, now time.Time, interval int) error {
	keys := make([]string, 0, interval)
	for i := 0; i < interval; i++ {
		keys = append(keys, hotMinuteKey(now.Add(-time.Duration(i)*time.Minute)))
	}
	pipe := r.redisClient.TxPipeline()
	pipe.ZUnionStore(ctx, mergeKey, &redis.ZStore{Keys: keys, Aggregate: "SUM"})
	pipe.Expire(ctx, mergeKey, hotMergeTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// RemoveFromHotBuckets 把视频从最近若干分钟的桶里移除
func (r *Repo) RemoveFromHotBuckets(ctx context.Context, videoID uint64, minutes int) error {
	if minutes <= 0 {
		return nil
	}
	now := time.Now().UTC().Truncate(time.Minute)
	member := strconv.FormatUint(videoID, 10)
	pipe := r.redisClient.TxPipeline()
	for i := 0; i < minutes; i++ {
		pipe.ZRem(ctx, hotMinuteKey(now.Add(-time.Duration(i)*time.Minute)), member)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func hotMinuteKey(minute time.Time) string {
	return fmt.Sprintf("%s:%s", hotMinutePrefix, minute.UTC().Format("200601021504"))
}

func hotMergeKey(minute time.Time, interval int) string {
	return fmt.Sprintf("%s:%d:%s", hotMergePrefix, interval, minute.UTC().Format("200601021504"))
}

func hotMarkKey(scoreType string, videoID uint64, userID uint64) string {
	return fmt.Sprintf("%s:%s:%d:%d", hotMarkPrefix, scoreType, videoID, userID)
}
