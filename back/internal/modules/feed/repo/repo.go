package repo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	hotMinutePrefix = "feed:hot:video:1m"
	hotMergePrefix  = "feed:hot:video:merge"
	hotMarkPrefix   = "feed:hot:mark"

	// 关注流的收件箱与发件箱。收件箱存小号推过来的内容，发件箱只有大号才有，
	// 读取时按发件箱键是否存在判断该作者的内容是否需要现取
	followInboxPrefix  = "feed:inbox"
	followOutboxPrefix = "feed:outbox"

	// HotRetentionMinutes 热度数据的保留分钟数。分钟桶的存活时间与查询窗口的上限
	// 都由它决定，两者取同一个值，声明的窗口不会超过实际留得住的数据
	HotRetentionMinutes = 70

	hotMinuteTTL = HotRetentionMinutes * time.Minute
	hotMergeTTL  = 2 * time.Minute

	// 关注流索引的保留时长与条数上限，超出上限的旧内容读不到
	followIndexTTL     = 30 * 24 * time.Hour
	followInboxMaxLen  = 1000
	followOutboxMaxLen = 500
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

// InboxItem 收件箱里的一条索引
type InboxItem struct {
	VideoID     uint64
	AuthorID    uint64
	PublishedAt time.Time
}

// inboxMember 收件箱成员带上作者 ID，读取时按关注状态过滤已取关的人
func inboxMember(videoID uint64, authorID uint64) string {
	return fmt.Sprintf("%d:%d", videoID, authorID)
}

// outboxMember 发件箱的键名已经表明作者，成员只需要视频 ID
func outboxMember(videoID uint64) string {
	return strconv.FormatUint(videoID, 10)
}

// followIndexScore 索引用发布时间当分值，同一毫秒内的先后由读取时用视频 ID 判定
func followIndexScore(publishedAt time.Time) float64 {
	return float64(publishedAt.UnixMilli())
}

func followInboxKey(userID uint64) string {
	return fmt.Sprintf("%s:%d", followInboxPrefix, userID)
}

func followOutboxKey(authorID uint64) string {
	return fmt.Sprintf("%s:%d", followOutboxPrefix, authorID)
}

// PushInboxItems 把一条视频推给一批粉丝的收件箱，同一批放进一个管道
func (r *Repo) PushInboxItems(ctx context.Context, followerIDs []uint64, item InboxItem) error {
	if len(followerIDs) == 0 || item.VideoID == 0 || item.PublishedAt.IsZero() {
		return nil
	}
	member := inboxMember(item.VideoID, item.AuthorID)
	score := followIndexScore(item.PublishedAt)
	pipe := r.redisClient.Pipeline()
	for _, followerID := range followerIDs {
		key := followInboxKey(followerID)
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
		pipe.ZRemRangeByRank(ctx, key, 0, -followInboxMaxLen-1)
		pipe.Expire(ctx, key, followIndexTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// PushInboxItemsForFollower 把一批视频写进同一个粉丝的收件箱，关注时回填历史内容用
func (r *Repo) PushInboxItemsForFollower(ctx context.Context, followerID uint64, items []InboxItem) error {
	if followerID == 0 || len(items) == 0 {
		return nil
	}
	key := followInboxKey(followerID)
	pipe := r.redisClient.Pipeline()
	for _, item := range items {
		if item.VideoID == 0 || item.PublishedAt.IsZero() {
			continue
		}
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  followIndexScore(item.PublishedAt),
			Member: inboxMember(item.VideoID, item.AuthorID),
		})
	}
	pipe.ZRemRangeByRank(ctx, key, 0, -followInboxMaxLen-1)
	pipe.Expire(ctx, key, followIndexTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// PushOutboxItem 把一条视频写进作者自己的发件箱，只有大号才会调用
func (r *Repo) PushOutboxItem(ctx context.Context, item InboxItem) error {
	if item.VideoID == 0 || item.AuthorID == 0 || item.PublishedAt.IsZero() {
		return nil
	}
	key := followOutboxKey(item.AuthorID)
	pipe := r.redisClient.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  followIndexScore(item.PublishedAt),
		Member: outboxMember(item.VideoID),
	})
	pipe.ZRemRangeByRank(ctx, key, 0, -followOutboxMaxLen-1)
	pipe.Expire(ctx, key, followIndexTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// HasOutbox 判断这批作者有没有发件箱，有就说明他是大号，读取时要去取他的发件箱
func (r *Repo) HasOutbox(ctx context.Context, authorIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(authorIDs))
	if len(authorIDs) == 0 {
		return result, nil
	}
	pipe := r.redisClient.Pipeline()
	commands := make([]*redis.IntCmd, len(authorIDs))
	for i, authorID := range authorIDs {
		commands[i] = pipe.Exists(ctx, followOutboxKey(authorID))
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	for i, authorID := range authorIDs {
		count, err := commands[i].Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		result[authorID] = count > 0
	}
	return result, nil
}

// FollowPage 读关注流的一页：我的收件箱，加上我关注的那些有大号的发件箱。
// 返回按时间倒序的索引与"索引是否可用"，全部为空时上层退回查表
func (r *Repo) FollowPage(ctx context.Context, userID uint64, followedIDs []uint64, lastCreatedAt time.Time, lastID uint64, limit int) ([]HotItem, bool, error) {
	if userID == 0 || limit <= 0 {
		return nil, false, nil
	}
	maxScore := "+inf"
	if lastID > 0 {
		maxScore = strconv.FormatInt(lastCreatedAt.UnixMilli(), 10)
	}

	followed := make(map[uint64]bool, len(followedIDs))
	candidates := make([]uint64, 0, len(followedIDs))
	for _, authorID := range followedIDs {
		followed[authorID] = true
		if authorID > 0 && authorID != userID {
			candidates = append(candidates, authorID)
		}
	}

	// 有发件箱的作者就是大号，他的内容不在我的收件箱里，要现取
	hasOutbox, err := r.HasOutbox(ctx, candidates)
	if err != nil {
		return nil, false, err
	}
	keys := []string{followInboxKey(userID)}
	ownerByKey := make(map[string]uint64, len(candidates))
	for _, authorID := range candidates {
		if !hasOutbox[authorID] {
			continue
		}
		key := followOutboxKey(authorID)
		ownerByKey[key] = authorID
		keys = append(keys, key)
	}

	pipe := r.redisClient.Pipeline()
	ranges := make([]*redis.ZSliceCmd, len(keys))
	for i, key := range keys {
		ranges[i] = pipe.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
			Min:   "-inf",
			Max:   maxScore,
			Count: int64(limit),
		})
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, false, err
	}

	items := make([]HotItem, 0, limit*len(keys))
	seen := make(map[uint64]bool, limit*len(keys))
	available := false
	for i, cmd := range ranges {
		values, err := cmd.Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, false, err
		}
		if len(values) == 0 {
			continue
		}
		available = true
		for _, value := range values {
			member, ok := value.Member.(string)
			if !ok {
				continue
			}
			videoID, authorID, ok := parseFollowMember(member, i == 0, ownerByKey[keys[i]])
			if !ok {
				continue
			}
			// 收件箱里的内容按当前关注状态过滤，取关之后残留的旧内容不再出现
			if !followed[authorID] {
				continue
			}
			if seen[videoID] {
				continue
			}
			// 游标是发布时间与视频 ID 两个字段，同一毫秒内的内容不会重复也不会漏
			if lastID > 0 && value.Score == float64(lastCreatedAt.UnixMilli()) && videoID >= lastID {
				continue
			}
			seen[videoID] = true
			items = append(items, HotItem{ID: videoID, Score: value.Score})
		}
	}
	if !available {
		return nil, false, nil
	}

	sort.Slice(items, func(a, b int) bool {
		if items[a].Score != items[b].Score {
			return items[a].Score > items[b].Score
		}
		return items[a].ID > items[b].ID
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, true, nil
}

// parseFollowMember 解析索引成员。收件箱的成员是"视频ID:作者ID"，
// 发件箱的成员只有视频 ID，作者从键名对应的那位取出
func parseFollowMember(member string, isInbox bool, outboxOwner uint64) (uint64, uint64, bool) {
	if !isInbox {
		videoID, err := strconv.ParseUint(member, 10, 64)
		if err != nil || outboxOwner == 0 {
			return 0, 0, false
		}
		return videoID, outboxOwner, true
	}
	parts := strings.SplitN(member, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	videoID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	authorID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return videoID, authorID, true
}
