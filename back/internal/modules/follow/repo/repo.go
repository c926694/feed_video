package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const followKeyFormat = "follow:%d"

// followerKeyFormat 反向集合：键是被人关注的一方，成员是关注他的人。
// 扇出要按作者枚举粉丝、判断作者规模，都靠这个集合
const followerKeyFormat = "follower:%d"

// Follow follow 表
type Follow struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Following uint64    `gorm:"not null;uniqueIndex:idx_user_follow" json:"user_id"`
	Follower  uint64    `gorm:"not null;uniqueIndex:idx_user_follow" json:"follow_user_id"`
	CreateTime time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP(3)" json:"created_at"`
}

// ListItem 关注或粉丝列表里的一项，昵称头像取自 user 表
type ListItem struct {
	RelationID uint64 `gorm:"column:relation_id"`
	UserID     uint64 `gorm:"column:user_id"`
	Nickname   string `gorm:"column:nickname"`
	AvatarURL  string `gorm:"column:avatar_url"`
}

// Repo 关注关系的读写，集合在 Redis，关系表在 MySQL
type Repo struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func New(db *gorm.DB, redisClient *redis.Client) *Repo {
	return &Repo{db: db, redisClient: redisClient}
}

// setScript 一段脚本同时维护两个方向的集合，状态本来就是目标态时不写入。
// KEYS[1] follow:{关注者} KEYS[2] follower:{被关注者}
// ARGV[1] 被关注者 ARGV[2] 目标态 ARGV[3] 关注者
var setScript = redis.NewScript(`
local want = tonumber(ARGV[2])
local cur = redis.call("SISMEMBER", KEYS[1], ARGV[1])
if cur == want then
    return 0
end
if want == 1 then
    redis.call("SADD", KEYS[1], ARGV[1])
    redis.call("SADD", KEYS[2], ARGV[3])
else
    redis.call("SREM", KEYS[1], ARGV[1])
    redis.call("SREM", KEYS[2], ARGV[3])
end
return 1
`)

// Set 把关注状态设置成目标态，返回状态是否发生了变化。
// 执行完之后两个方向的集合都等于目标态
func (r *Repo) Set(ctx context.Context, follower uint64, following uint64, active bool) (bool, error) {
	val := "0"
	if active {
		val = "1"
	}
	result, err := setScript.Run(ctx, r.redisClient,
		[]string{followKey(follower), followerKey(following)},
		following, val, follower).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// Create 写入一条关注关系，冲突时忽略，保证事件重放幂等
func (r *Repo) Create(ctx context.Context, follower uint64, following uint64) error {
	item := Follow{Following: following, Follower: follower}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error
}

// UserExists 判断用户是否存在，关注前校验目标
func (r *Repo) UserExists(ctx context.Context, userID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("user").Where("id = ?", userID).Count(&count).Error
	return count > 0, err
}

func (r *Repo) Delete(ctx context.Context, follower uint64, following uint64) error {
	return r.db.WithContext(ctx).
		Where("follower = ? and following = ?", follower, following).
		Delete(&Follow{}).Error
}

// AppendFollowing 把这一批用户的关注状态查询追加到给定管道，供调用方合并成一次往返
func (r *Repo) AppendFollowing(pipe redis.Pipeliner, ctx context.Context, follower uint64, followingIDs []uint64) []*redis.BoolCmd {
	key := followKey(follower)
	commands := make([]*redis.BoolCmd, len(followingIDs))
	for i, followingID := range followingIDs {
		commands[i] = pipe.SIsMember(ctx, key, followingID)
	}
	return commands
}

// FilterFollowing 批量查询用户是否关注了这些人
func (r *Repo) FilterFollowing(ctx context.Context, follower uint64, followingIDs []uint64) (map[uint64]bool, error) {
	result := make(map[uint64]bool, len(followingIDs))
	if len(followingIDs) == 0 {
		return result, nil
	}
	pipeline := r.redisClient.Pipeline()
	commands := r.AppendFollowing(pipeline, ctx, follower, followingIDs)
	if _, err := pipeline.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	for i, followingID := range followingIDs {
		followed, err := commands[i].Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		result[followingID] = followed
	}
	return result, nil
}

// FollowingIDs 取用户关注的全部人
func (r *Repo) FollowingIDs(ctx context.Context, follower uint64) ([]uint64, error) {
	members, err := r.redisClient.SMembers(ctx, followKey(follower)).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(members))
	for _, member := range members {
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil {
			return nil, parseErr
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func followKey(follower uint64) string {
	return fmt.Sprintf(followKeyFormat, follower)
}

func followerKey(following uint64) string {
	return fmt.Sprintf(followerKeyFormat, following)
}

// CountFollowing 统计用户关注的人数
func (r *Repo) CountFollowing(ctx context.Context, follower uint64) (int64, error) {
	return r.redisClient.SCard(ctx, followKey(follower)).Result()
}

// CountFollowers 统计用户的粉丝数。取反向集合的基数，与关注数同源，
// 都来自关注接口同步写入的状态
func (r *Repo) CountFollowers(ctx context.Context, following uint64) (int64, error) {
	return r.redisClient.SCard(ctx, followerKey(following)).Result()
}

// FollowerIDsPage 用 SSCAN 分页取粉丝，供扇出时逐批写收件箱。
// 返回本次取到的粉丝与下一次的游标，游标为 0 表示已经扫完
func (r *Repo) FollowerIDsPage(ctx context.Context, following uint64, cursor uint64, count int64) ([]uint64, uint64, error) {
	members, next, err := r.redisClient.SScan(ctx, followerKey(following), cursor, "", count).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, 0, err
	}
	ids := make([]uint64, 0, len(members))
	for _, member := range members {
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil {
			return nil, 0, parseErr
		}
		ids = append(ids, id)
	}
	return ids, next, nil
}

// FollowingPage 关注列表：这个人关注了谁，按关系 ID 倒序（最近关注的在前面）。
// 内连接 user 表，已经没有账号的关系行不会出现
func (r *Repo) FollowingPage(ctx context.Context, follower uint64, lastID uint64, limit int) ([]ListItem, error) {
	items := make([]ListItem, 0, limit)
	query := r.db.WithContext(ctx).
		Table("follow AS f").
		Select("f.id AS relation_id, u.id AS user_id, u.nick_name AS nickname, u.avatar_url AS avatar_url").
		Joins("JOIN user AS u ON u.id = f.following").
		Where("f.follower = ?", follower)
	if lastID > 0 {
		query = query.Where("f.id < ?", lastID)
	}
	err := query.Order("f.id desc").Limit(limit).Scan(&items).Error
	return items, err
}

// FollowersPage 粉丝列表：谁关注了这个人，按关系 ID 倒序（最近关注的在前面）
func (r *Repo) FollowersPage(ctx context.Context, following uint64, lastID uint64, limit int) ([]ListItem, error) {
	items := make([]ListItem, 0, limit)
	query := r.db.WithContext(ctx).
		Table("follow AS f").
		Select("f.id AS relation_id, u.id AS user_id, u.nick_name AS nickname, u.avatar_url AS avatar_url").
		Joins("JOIN user AS u ON u.id = f.follower").
		Where("f.following = ?", following)
	if lastID > 0 {
		query = query.Where("f.id < ?", lastID)
	}
	err := query.Order("f.id desc").Limit(limit).Scan(&items).Error
	return items, err
}

// RestoreSets 从关注表分页扫出全部关系，重建两个方向的集合。
// Redis 数据丢失后靠它恢复，集合本身只有关注接口一个写入方
func (r *Repo) RestoreSets(ctx context.Context) (int, error) {
	const pageSize = 1000
	restored := 0
	lastID := uint64(0)
	for {
		rows := make([]Follow, 0, pageSize)
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).
			Order("id asc").Limit(pageSize).Find(&rows).Error; err != nil {
			return restored, err
		}
		if len(rows) == 0 {
			return restored, nil
		}
		pipe := r.redisClient.Pipeline()
		for _, row := range rows {
			pipe.SAdd(ctx, followKey(row.Follower), row.Following)
			pipe.SAdd(ctx, followerKey(row.Following), row.Follower)
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return restored, err
		}
		restored += len(rows)
		lastID = rows[len(rows)-1].ID
		if len(rows) < pageSize {
			return restored, nil
		}
	}
}
