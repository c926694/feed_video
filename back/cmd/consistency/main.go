package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"simple_tiktok/internal/modules/message"
	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/mysql"
	platformredis "simple_tiktok/internal/platform/redis"
)

// 数据一致性迁移：按 Redis 里作为权威状态的集合修正 MySQL 里的关系表与计数。
//
// 关系表只补不删：集合里有而表里没有的补齐，集合里已经有的不重复插入。
// 计数一律按设计口径重算：点赞、收藏、关注相关的计数以集合为准，
// 视频数、评论数、回复数以表内实际行数为准。
func main() {
	dryRun := flag.Bool("dry-run", false, "只统计将要修复的数量，不写数据")
	flag.Parse()

	path, err := resolveConfigPath()
	if err != nil {
		log.Fatalf("定位配置文件失败: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Println("配置文件:", path, "dry-run:", *dryRun)

	db, err := mysql.Connect(cfg.MySQL)
	if err != nil {
		log.Fatalf("连接 mysql 失败: %v", err)
	}
	defer func() {
		if closeErr := mysql.Close(db); closeErr != nil {
			log.Printf("关闭 mysql 失败: %v", closeErr)
		}
	}()

	redisClient, err := platformredis.Connect(cfg.Redis)
	if err != nil {
		log.Fatalf("连接 redis 失败: %v", err)
	}
	defer func() {
		if closeErr := platformredis.Close(redisClient); closeErr != nil {
			log.Printf("关闭 redis 失败: %v", closeErr)
		}
	}()

	ctx := context.Background()
	runner := &migration{db: db, redisClient: redisClient, dryRun: *dryRun}

	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		{"补齐点赞关系表", runner.syncLikeRows},
		{"补齐收藏关系表", runner.syncFavoriteRows},
		{"修正关注关系", runner.syncFollowRows},
		{"重算点赞与收藏计数", runner.rebuildInteractionCounts},
		{"重算关注计数", runner.rebuildFollowCounts},
		{"重算视频数与评论数", runner.rebuildContentCounts},
		{"补齐站内通知", runner.backfillMessages},
		{"检查孤儿数据", runner.reportOrphans},
	}
	for _, step := range steps {
		if err = step.run(ctx); err != nil {
			log.Fatalf("%s 失败: %v", step.name, err)
		}
	}
	for _, line := range runner.lines {
		log.Println(line)
	}
	if *dryRun {
		log.Println("dry-run 结束，没有写入任何数据")
		return
	}
	log.Println("一致性迁移完成")
}

type migration struct {
	db          *gorm.DB
	redisClient *redis.Client
	dryRun      bool
	lines       []string
}

func (m *migration) report(format string, args ...any) {
	m.lines = append(m.lines, fmt.Sprintf(format, args...))
}

// scanKeys 按模式扫出全部键
func (m *migration) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	keys := make([]string, 0, 64)
	cursor := uint64(0)
	for {
		batch, next, err := m.redisClient.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		if next == 0 {
			return keys, nil
		}
		cursor = next
	}
}

// parseKeyID 从 "前缀:{id}" 这样的键名里取出 ID
func parseKeyID(key string) (uint64, bool) {
	index := strings.LastIndex(key, ":")
	if index < 0 {
		return 0, false
	}
	id, err := strconv.ParseUint(key[index+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (m *migration) members(ctx context.Context, key string) ([]uint64, error) {
	values, err := m.redisClient.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(values))
	for _, value := range values {
		id, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// exists 判断关系行是否已经存在，dry-run 用它统计缺口
func (m *migration) exists(ctx context.Context, table string, where string, args ...any) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Table(table).Where(where, args...).Count(&count).Error
	return count > 0, err
}

// syncLikeRows 把 Redis 点赞集合里有的成员补进 user_like
func (m *migration) syncLikeRows(ctx context.Context) error {
	synced := 0
	for _, item := range []struct {
		pattern    string
		targetType string
	}{
		{"like:video:*", "video"},
		{"like:comment:*", "comment"},
	} {
		keys, err := m.scanKeys(ctx, item.pattern)
		if err != nil {
			return err
		}
		for _, key := range keys {
			targetID, ok := parseKeyID(key)
			if !ok {
				continue
			}
			userIDs, err := m.members(ctx, key)
			if err != nil {
				return err
			}
			for _, userID := range userIDs {
				if m.dryRun {
					found, err := m.exists(ctx, "user_like",
						"user_id = ? AND target_type = ? AND target_id = ?", userID, item.targetType, targetID)
					if err != nil {
						return err
					}
					if !found {
						synced++
					}
					continue
				}
				result := m.db.WithContext(ctx).Exec(
					"INSERT INTO user_like (user_id, target_type, target_id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE id = id",
					userID, item.targetType, targetID)
				if result.Error != nil {
					return result.Error
				}
				synced += int(result.RowsAffected)
			}
		}
	}
	m.report("点赞关系表：补齐 %d 条", synced)
	return nil
}

// syncFavoriteRows 把 Redis 收藏集合里有的成员补进 user_favorite
func (m *migration) syncFavoriteRows(ctx context.Context) error {
	synced := 0
	keys, err := m.scanKeys(ctx, "favorite:video:*")
	if err != nil {
		return err
	}
	for _, key := range keys {
		videoID, ok := parseKeyID(key)
		if !ok {
			continue
		}
		userIDs, err := m.members(ctx, key)
		if err != nil {
			return err
		}
		for _, userID := range userIDs {
			if m.dryRun {
				found, err := m.exists(ctx, "user_favorite", "user_id = ? AND video_id = ?", userID, videoID)
				if err != nil {
					return err
				}
				if !found {
					synced++
				}
				continue
			}
			result := m.db.WithContext(ctx).Exec(
				"INSERT INTO user_favorite (user_id, video_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE id = id",
				userID, videoID)
			if result.Error != nil {
				return result.Error
			}
			synced += int(result.RowsAffected)
		}
	}
	m.report("收藏关系表：补齐 %d 条", synced)
	return nil
}

// syncFollowRows 两个方向互相补齐：集合里有的补进 follow 表，表里有的补进两个集合。
// 集合里成员是"关注的人"，所以键上的 ID 是关注者
func (m *migration) syncFollowRows(ctx context.Context) error {
	inserted := 0
	keys, err := m.scanKeys(ctx, "follow:*")
	if err != nil {
		return err
	}
	for _, key := range keys {
		follower, ok := parseKeyID(key)
		if !ok {
			continue
		}
		followingIDs, err := m.members(ctx, key)
		if err != nil {
			return err
		}
		for _, following := range followingIDs {
			if m.dryRun {
				found, err := m.exists(ctx, "follow", "follower = ? AND following = ?", follower, following)
				if err != nil {
					return err
				}
				if !found {
					inserted++
				}
				continue
			}
			result := m.db.WithContext(ctx).Exec(
				"INSERT INTO follow (follower, following) VALUES (?, ?) ON DUPLICATE KEY UPDATE id = id",
				follower, following)
			if result.Error != nil {
				return result.Error
			}
			inserted += int(result.RowsAffected)
		}
	}

	restored := 0
	if !m.dryRun {
		rows := make([]struct {
			Follower  uint64
			Following uint64
		}, 0, 1000)
		if err = m.db.WithContext(ctx).Table("follow").
			Select("follower, following").Order("id asc").Scan(&rows).Error; err != nil {
			return err
		}
		pipe := m.redisClient.Pipeline()
		for _, row := range rows {
			pipe.SAdd(ctx, fmt.Sprintf("follow:%d", row.Follower), row.Following)
			pipe.SAdd(ctx, fmt.Sprintf("follower:%d", row.Following), row.Follower)
		}
		if _, err = pipe.Exec(ctx); err != nil && err != redis.Nil {
			return err
		}
		restored = len(rows)
	}
	m.report("关注关系：表补齐 %d 条，集合按表回灌 %d 条关系", inserted, restored)
	return nil
}

// rebuildInteractionCounts 点赞与收藏的计数按集合大小重算
func (m *migration) rebuildInteractionCounts(ctx context.Context) error {
	videoLikes, err := m.recount(ctx, "like:video:*", "video", "like_count")
	if err != nil {
		return err
	}
	commentLikes, err := m.recount(ctx, "like:comment:*", "comment", "like_count")
	if err != nil {
		return err
	}
	videoFavorites, err := m.recount(ctx, "favorite:video:*", "video", "favorite_count")
	if err != nil {
		return err
	}
	m.report("计数：视频点赞 %d 行，评论点赞 %d 行，视频收藏 %d 行", videoLikes, commentLikes, videoFavorites)
	return nil
}

// recount 把某张表的计数字段按对应集合的大小重算，没有集合的行归零
func (m *migration) recount(ctx context.Context, pattern string, table string, column string) (int, error) {
	keys, err := m.scanKeys(ctx, pattern)
	if err != nil {
		return 0, err
	}
	counted := map[uint64]int64{}
	for _, key := range keys {
		id, ok := parseKeyID(key)
		if !ok {
			continue
		}
		size, err := m.redisClient.SCard(ctx, key).Result()
		if err != nil {
			return 0, err
		}
		counted[id] = size
	}

	changed := 0
	if m.dryRun {
		rows := make([]struct {
			ID    uint64
			Value int64
		}, 0, 1000)
		if err = m.db.WithContext(ctx).Table(table).Select("id, "+column+" as value").Scan(&rows).Error; err != nil {
			return 0, err
		}
		for _, row := range rows {
			if counted[row.ID] != row.Value {
				changed++
			}
		}
		return changed, nil
	}

	for id, size := range counted {
		result := m.db.WithContext(ctx).Table(table).Where("id = ? AND "+column+" <> ?", id, size).
			Update(column, size)
		if result.Error != nil {
			return 0, result.Error
		}
		changed += int(result.RowsAffected)
	}
	// 集合已经不存在但计数还留着的行归零
	query := m.db.WithContext(ctx).Table(table).Where(column+" <> 0")
	if len(counted) > 0 {
		ids := make([]uint64, 0, len(counted))
		for id := range counted {
			ids = append(ids, id)
		}
		query = query.Where("id NOT IN ?", ids)
	}
	result := query.Update(column, 0)
	if result.Error != nil {
		return 0, result.Error
	}
	return changed + int(result.RowsAffected), nil
}

// rebuildFollowCounts 关注数与粉丝数按两个方向的集合大小重算
func (m *migration) rebuildFollowCounts(ctx context.Context) error {
	following, err := m.recount(ctx, "follow:*", "user", "follow_count")
	if err != nil {
		return err
	}
	followers, err := m.recount(ctx, "follower:*", "user", "follower_count")
	if err != nil {
		return err
	}
	m.report("计数：关注数 %d 行，粉丝数 %d 行", following, followers)
	return nil
}

// rebuildContentCounts 视频数、评论数、回复数按表内实际行数重算
func (m *migration) rebuildContentCounts(ctx context.Context) error {
	videoCountSQL := `
UPDATE user u
LEFT JOIN (
  SELECT author_id, COUNT(*) AS total FROM video WHERE status = 'published' GROUP BY author_id
) x ON x.author_id = u.id
SET u.video_count = COALESCE(x.total, 0)
WHERE u.video_count <> COALESCE(x.total, 0)`
	commentCountSQL := `
UPDATE video v
LEFT JOIN (
  SELECT video_id, COUNT(*) AS total FROM comment GROUP BY video_id
) x ON x.video_id = v.id
SET v.comment_count = COALESCE(x.total, 0)
WHERE v.comment_count <> COALESCE(x.total, 0)`
	replyCountSQL := `
UPDATE comment c
LEFT JOIN (
  SELECT parent_id, COUNT(*) AS total FROM comment WHERE parent_id > 0 GROUP BY parent_id
) x ON x.parent_id = c.id
SET c.reply_count = COALESCE(x.total, 0)
WHERE c.reply_count <> COALESCE(x.total, 0)`

	if m.dryRun {
		videoCount, err := m.countDirty(ctx, "SELECT COUNT(*) FROM user u LEFT JOIN (SELECT author_id, COUNT(*) AS total FROM video WHERE status = 'published' GROUP BY author_id) x ON x.author_id = u.id WHERE u.video_count <> COALESCE(x.total, 0)")
		if err != nil {
			return err
		}
		commentCount, err := m.countDirty(ctx, "SELECT COUNT(*) FROM video v LEFT JOIN (SELECT video_id, COUNT(*) AS total FROM comment GROUP BY video_id) x ON x.video_id = v.id WHERE v.comment_count <> COALESCE(x.total, 0)")
		if err != nil {
			return err
		}
		replyCount, err := m.countDirty(ctx, "SELECT COUNT(*) FROM comment c LEFT JOIN (SELECT parent_id, COUNT(*) AS total FROM comment WHERE parent_id > 0 GROUP BY parent_id) x ON x.parent_id = c.id WHERE c.reply_count <> COALESCE(x.total, 0)")
		if err != nil {
			return err
		}
		m.report("计数：用户视频数 %d 行，视频评论数 %d 行，评论回复数 %d 行", videoCount, commentCount, replyCount)
		return nil
	}

	videoCount, err := m.exec(ctx, videoCountSQL)
	if err != nil {
		return err
	}
	commentCount, err := m.exec(ctx, commentCountSQL)
	if err != nil {
		return err
	}
	replyCount, err := m.exec(ctx, replyCountSQL)
	if err != nil {
		return err
	}
	m.report("计数：用户视频数 %d 行，视频评论数 %d 行，评论回复数 %d 行", videoCount, commentCount, replyCount)
	return nil
}

func (m *migration) exec(ctx context.Context, query string) (int, error) {
	result := m.db.WithContext(ctx).Exec(query)
	return int(result.RowsAffected), result.Error
}

// backfillMessages 按现有的点赞、评论、关注数据补齐站内通知。
// 通知明细的唯一键保证同一个人对同一个目标只会有一条，命令重复执行不会产生重复通知
func (m *migration) backfillMessages(ctx context.Context) error {
	logic := message.NewMaintenanceLogic(m.db, m.redisClient)
	stats, err := logic.Backfill(ctx, 500)
	if err != nil {
		return err
	}
	m.report("站内通知：新补点赞视频 %d 条、点赞评论 %d 条、评论 %d 条、回复 %d 条、关注 %d 条；跳过 %d 条（自己操作自己或目标已不存在）、%d 条（接收者已经没有账号）",
		stats.LikeVideo, stats.LikeComment, stats.Comment, stats.Reply, stats.Follow,
		stats.SkippedMissing, stats.SkippedActor)
	return nil
}

// reportOrphans 只统计指向已不存在对象的记录，不删除。
// 要不要清理是产品决定，所以迁移只把它列出来
func (m *migration) reportOrphans(ctx context.Context) error {
	checks := []struct {
		name  string
		query string
	}{
		{"关注关系指向不存在的被关注者", "SELECT COUNT(*) FROM follow f LEFT JOIN user u ON u.id = f.following WHERE u.id IS NULL"},
		{"关注关系里的关注者不存在", "SELECT COUNT(*) FROM follow f LEFT JOIN user u ON u.id = f.follower WHERE u.id IS NULL"},
		{"视频的作者不存在", "SELECT COUNT(*) FROM video v LEFT JOIN user u ON u.id = v.author_id WHERE u.id IS NULL"},
		{"评论所属的视频不存在", "SELECT COUNT(*) FROM comment c LEFT JOIN video v ON v.id = c.video_id WHERE v.id IS NULL"},
		{"评论的父评论不存在", "SELECT COUNT(*) FROM comment c LEFT JOIN comment p ON p.id = c.parent_id WHERE c.parent_id > 0 AND p.id IS NULL"},
		{"点赞关系里的用户不存在", "SELECT COUNT(*) FROM user_like l LEFT JOIN user u ON u.id = l.user_id WHERE u.id IS NULL"},
		{"收藏关系里的用户或视频不存在", "SELECT COUNT(*) FROM user_favorite f LEFT JOIN user u ON u.id = f.user_id LEFT JOIN video v ON v.id = f.video_id WHERE u.id IS NULL OR v.id IS NULL"},
	}
	total := 0
	for _, check := range checks {
		count, err := m.countDirty(ctx, check.query)
		if err != nil {
			return err
		}
		if count == 0 {
			continue
		}
		total += count
		m.report("孤儿数据：%s %d 条（迁移不删除，需要人工确认）", check.name, count)
	}
	if total == 0 {
		m.report("孤儿数据：没有发现")
	}
	return nil
}

func (m *migration) countDirty(ctx context.Context, query string) (int, error) {
	var count int
	err := m.db.WithContext(ctx).Raw(query).Scan(&count).Error
	return count, err
}

// resolveConfigPath 从当前工作目录逐级向上查找配置文件
func resolveConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("获取绝对路径失败: %w", err)
	}
	for {
		candidate := filepath.Join(dir, config.DefaultRelativePath)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("没有找到 %s", config.DefaultRelativePath)
}
