package video

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	commentevent "simple_tiktok/internal/modules/comment/event"
	commentrepo "simple_tiktok/internal/modules/comment/repo"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	likeevent "simple_tiktok/internal/modules/like/event"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userevent "simple_tiktok/internal/modules/user/event"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/platform/sts"
	"simple_tiktok/internal/platform/upload"
)

const (
	infoLogicalTTL     = 5 * time.Minute
	infoNullLogicalTTL = 2 * time.Minute
	infoRebuildLockTTL = 10 * time.Second
	infoMissRetryTimes = 8
	infoMissRetrySleep = 30 * time.Millisecond

	coverMaxSize = 10 << 20 // 封面最大 10MB
	videoMaxSize = 10 << 30 // 视频最大 10GB
)

// Logic 视频模块的业务逻辑
type Logic struct {
	videos   *videorepo.Repo
	comments *commentrepo.Repo
	users    *userrepo.Repo
	likes    *likerepo.Repo
	follows  *followrepo.Repo
	producer *producer.Producer
	uploader *upload.Uploader
	sts      *sts.Service
}

// CreateVideo 创建发布记录，状态为已创建，不发事件。文件由前端直传 OSS。
// 带请求 ID 的重复提交返回冲突错误并带上已创建的记录。
func (l *Logic) CreateVideo(ctx context.Context, createReq CreateReq, userID uint64) (CreateRes, error) {
	if err := validateUploadKey(upload.Cover, createReq.CoverKey, userID); err != nil {
		return CreateRes{}, err
	}
	if err := validateUploadKey(upload.Video, createReq.PlayKey, userID); err != nil {
		return CreateRes{}, err
	}

	// 作者展示字段来自 user 表，不取令牌里的昵称，避免改昵称之后显示成旧值
	user, err := l.users.GetByID(ctx, userID)
	if err != nil {
		return CreateRes{}, err
	}

	requestID := createReq.RequestId
	if requestID == "" {
		// 未带请求 ID 时不参与判重，生成随机值占位
		if requestID, err = randomRequestID(); err != nil {
			return CreateRes{}, err
		}
	}

	item := videorepo.Video{
		Title:        createReq.Title,
		Description:  createReq.Description,
		AuthorID:     userID,
		AuthorName:   user.NickName,
		AuthorAvatar: user.AvatarURL,
		PlayURL:      createReq.PlayKey,
		CoverURL:     createReq.CoverKey,
		Status:       videorepo.StatusCreated,
		RequestID:    requestID,
	}
	if err = l.videos.Create(ctx, &item); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 重复创建：返回冲突错误并带上已创建的记录
			existing, getErr := l.videos.GetByRequestID(ctx, userID, requestID)
			if getErr != nil {
				return CreateRes{}, getErr
			}
			return CreateRes{Id: existing.ID, Status: existing.Status}, httpx.New(httpx.CodeConflict, "该发布记录已创建，禁止重复创建")
		}
		return CreateRes{}, err
	}
	return CreateRes{Id: item.ID, Status: item.Status}, nil
}

// randomRequestID 生成随机请求 ID，占位不参与判重
func randomRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// UploadCredential 签发直传凭证并生成存储路径
func (l *Logic) UploadCredential(ctx context.Context, userID uint64, credentialReq CredentialReq) (*CredentialRes, error) {
	if l.sts == nil {
		return nil, httpx.New(httpx.CodeInternal, "直传服务未配置")
	}
	coverKey, err := l.uploader.BuildKey(upload.Cover, userID, credentialReq.CoverExt)
	if err != nil {
		return nil, err
	}
	playKey, err := l.uploader.BuildKey(upload.Video, userID, credentialReq.PlayExt)
	if err != nil {
		return nil, err
	}
	return l.buildCredential(ctx, coverKey, playKey)
}

// buildCredential 签发直传凭证并组装响应
func (l *Logic) buildCredential(ctx context.Context, coverKey string, playKey string) (*CredentialRes, error) {
	credential, err := l.sts.Assume(ctx)
	if err != nil {
		return nil, err
	}
	return &CredentialRes{
		AccessKeyID:     credential.AccessKeyID,
		AccessKeySecret: credential.AccessKeySecret,
		SecurityToken:   credential.SecurityToken,
		Expiration:      credential.Expiration.Unix(),
		Region:          l.uploader.Region(),
		Bucket:          l.uploader.BucketName(),
		CoverKey:        coverKey,
		PlayKey:         playKey,
	}, nil
}

// UpdateStatus 更新发布状态：published 发布完成、failed 标记失败、created 重试
func (l *Logic) UpdateStatus(ctx context.Context, videoID uint64, userID uint64, status string) (*CredentialRes, error) {
	switch status {
	case videorepo.StatusPublished:
		if err := l.publish(ctx, videoID, userID); err != nil {
			return nil, err
		}
		return nil, nil
	case videorepo.StatusFailed:
		if _, err := l.markStatus(ctx, videoID, userID, videorepo.StatusCreated, videorepo.StatusFailed); err != nil {
			return nil, err
		}
		return nil, nil
	case videorepo.StatusCreated:
		if l.sts == nil {
			return nil, httpx.New(httpx.CodeInternal, "直传服务未配置")
		}
		item, err := l.markStatus(ctx, videoID, userID, videorepo.StatusFailed, videorepo.StatusCreated)
		if err != nil {
			return nil, err
		}
		return l.buildCredential(ctx, item.CoverURL, item.PlayURL)
	default:
		return nil, httpx.New(httpx.CodeBadRequest, "状态取值不合法")
	}
}

// publish 发布完成：校验对象存在与大小，把 created 迁移到 published 并发出事件
func (l *Logic) publish(ctx context.Context, videoID uint64, userID uint64) error {
	item, err := l.getOwnVideo(ctx, videoID, userID, "只能发布自己的视频")
	if err != nil {
		return err
	}
	playSize, err := l.uploader.Head(upload.Video, item.PlayURL)
	if err != nil {
		return httpx.New(httpx.CodeBadRequest, "视频文件还没上传完成")
	}
	if playSize <= 0 {
		return httpx.New(httpx.CodeBadRequest, "视频文件为空")
	}
	if playSize > videoMaxSize {
		return httpx.New(httpx.CodeBadRequest, "视频不能超过 10GB")
	}
	coverSize, err := l.uploader.Head(upload.Cover, item.CoverURL)
	if err != nil {
		return httpx.New(httpx.CodeBadRequest, "封面还没上传完成")
	}
	if coverSize > coverMaxSize {
		return httpx.New(httpx.CodeBadRequest, "封面不能超过 10MB")
	}
	migrated, err := l.videos.MarkStatus(ctx, videoID, userID, videorepo.StatusCreated, videorepo.StatusPublished)
	if err != nil {
		return err
	}
	if !migrated {
		// 已经发布过，幂等返回成功，不重复发事件
		return nil
	}
	return l.producer.Publish(ctx, topic.VideoCreated, strconv.FormatUint(videoID, 10), videoevent.CreatedEvent{
		VideoID:   videoID,
		AuthorID:  userID,
		CreatedAt: item.CreateTime,
	})
}

// markStatus 校验归属后做状态迁移，返回迁移前的记录，迁移幂等
func (l *Logic) markStatus(ctx context.Context, videoID uint64, userID uint64, from string, to string) (*videorepo.Video, error) {
	item, err := l.getOwnVideo(ctx, videoID, userID, "只能操作自己的视频")
	if err != nil {
		return nil, err
	}
	if _, err = l.videos.MarkStatus(ctx, videoID, userID, from, to); err != nil {
		return nil, err
	}
	return item, nil
}

// getOwnVideo 查视频并校验属于当前用户
func (l *Logic) getOwnVideo(ctx context.Context, videoID uint64, userID uint64, forbiddenMsg string) (*videorepo.Video, error) {
	item, err := l.videos.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.New(httpx.CodeNotFound, "视频不存在")
		}
		return nil, err
	}
	if item.AuthorID != userID {
		return nil, httpx.New(httpx.CodeForbidden, forbiddenMsg)
	}
	return item, nil
}

// validateUploadKey 校验直传 key 的目录前缀与用户归属
func validateUploadKey(sourceType upload.SourceType, key string, userID uint64) error {
	prefix := ""
	switch sourceType {
	case upload.Cover:
		prefix = constants.CoverPrefix
	case upload.Video:
		prefix = constants.VideoPrefix
	default:
		return fmt.Errorf("不支持的上传类型 %s", sourceType)
	}
	if !strings.HasPrefix(key, prefix) {
		return httpx.New(httpx.CodeBadRequest, "存储路径不合法")
	}
	rest := strings.TrimPrefix(key, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return httpx.New(httpx.CodeBadRequest, "存储路径不合法")
	}
	owner, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || owner != userID {
		return httpx.New(httpx.CodeBadRequest, "存储路径不合法")
	}
	return nil
}

func (l *Logic) GetMyVideos(ctx context.Context, userID uint64, limit uint64) ([]InfoRes, error) {
	items, err := l.videos.ListByAuthor(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	list := make([]InfoRes, len(items))
	for i, item := range items {
		list[i] = l.toInfoRes(item)
	}
	if err = l.fillLiked(ctx, list, userID); err != nil {
		return nil, err
	}
	if err = l.fillFollowed(ctx, list, userID); err != nil {
		return nil, err
	}
	return list, nil
}

func (l *Logic) GetVideoInfo(ctx context.Context, videoID uint64, userID uint64) (InfoRes, error) {
	info, exists, err := l.getInfoWithCache(ctx, videoID)
	if err != nil {
		return InfoRes{}, err
	}
	if !exists {
		return InfoRes{}, httpx.New(httpx.CodeNotFound, "视频不存在")
	}
	if info.Status != videorepo.StatusPublished {
		return InfoRes{}, httpx.New(httpx.CodeNotFound, "视频不存在")
	}
	list := []InfoRes{info}
	if err = l.fillLiked(ctx, list, userID); err != nil {
		return InfoRes{}, err
	}
	if err = l.fillFollowed(ctx, list, userID); err != nil {
		return InfoRes{}, err
	}
	return list[0], nil
}

func (l *Logic) DeleteVideo(ctx context.Context, videoID uint64, userID uint64) error {
	item, err := l.videos.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.New(httpx.CodeNotFound, "视频不存在")
		}
		return err
	}
	if item.AuthorID != userID {
		return httpx.New(httpx.CodeForbidden, "只能删除自己发布的视频")
	}
	if err = l.videos.Delete(ctx, videoID); err != nil {
		return err
	}
	if err = l.videos.DeleteInfoCache(ctx, videoID); err != nil {
		slog.Error("清理视频信息缓存失败", "video_id", videoID, "error", err)
	}

	if item.Status != videorepo.StatusPublished {
		// 未发布过：没有事件订阅方需要清理，同步清掉可能已上传的对象
		if deleteErr := l.uploader.Delete(upload.Video, item.PlayURL); deleteErr != nil {
			slog.Error("清理视频文件失败", "video_id", videoID, "error", deleteErr)
		}
		if deleteErr := l.uploader.Delete(upload.Cover, item.CoverURL); deleteErr != nil {
			slog.Error("清理封面失败", "video_id", videoID, "error", deleteErr)
		}
		return nil
	}

	// 已发布：发事件，订阅方各自清理对象、评论、点赞、热度
	return l.producer.Publish(ctx, topic.VideoDeleted, strconv.FormatUint(videoID, 10), videoevent.DeletedEvent{
		VideoID:  videoID,
		AuthorID: item.AuthorID,
		PlayURL:  item.PlayURL,
		CoverURL: item.CoverURL,
	})
}

// HandleLikeSwitched 订阅点赞事件，只处理视频点赞
func (l *Logic) HandleLikeSwitched(ctx context.Context, payload []byte) error {
	var switched likeevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Target != likeevent.TargetVideo {
		return nil
	}
	if switched.TargetID == 0 {
		return consumer.Permanent(errors.New("点赞事件里没有 targetId"))
	}

	count, err := l.likes.CountLikes(ctx, likeevent.TargetVideo, switched.TargetID)
	if err != nil {
		slog.Error("统计视频点赞数失败", "video_id", switched.TargetID, "error", err)
		return err
	}
	if err = l.videos.SyncLikeCount(ctx, switched.TargetID, count); err != nil {
		slog.Error("对账视频点赞数失败", "video_id", switched.TargetID, "error", err)
		return err
	}
	return l.videos.DeleteInfoCache(ctx, switched.TargetID)
}

// HandleCommentCreated 订阅评论创建事件，按实际评论数对账
func (l *Logic) HandleCommentCreated(ctx context.Context, payload []byte) error {
	var created commentevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.VideoID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有 videoId"))
	}
	return l.syncCommentCount(ctx, created.VideoID)
}

// HandleCommentDeleted 订阅评论删除事件，按实际评论数对账
func (l *Logic) HandleCommentDeleted(ctx context.Context, payload []byte) error {
	var deleted commentevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有 videoId"))
	}
	return l.syncCommentCount(ctx, deleted.VideoID)
}

// syncCommentCount 按评论表实际行数对账视频评论数
func (l *Logic) syncCommentCount(ctx context.Context, videoID uint64) error {
	count, err := l.comments.CountByVideo(ctx, videoID)
	if err != nil {
		slog.Error("统计视频评论数失败", "video_id", videoID, "error", err)
		return err
	}
	if err = l.videos.SyncCommentCount(ctx, videoID, count); err != nil {
		slog.Error("对账视频评论数失败", "video_id", videoID, "error", err)
		return err
	}
	return l.videos.DeleteInfoCache(ctx, videoID)
}

func (l *Logic) toInfoRes(item videorepo.Video) InfoRes {
	return InfoRes{
		Id:           item.ID,
		AuthorID:     item.AuthorID,
		AuthorName:   item.AuthorName,
		AuthorAvatar: l.uploader.URL(item.AuthorAvatar),
		Title:        item.Title,
		Description:  item.Description,
		CoverURL:     l.uploader.URL(item.CoverURL),
		PlayURL:      l.uploader.URL(item.PlayURL),
		CommentCount: item.CommentCount,
		LikeCount:    item.LikeCount,
		Status:       item.Status,
		CreatedAt:    item.CreateTime,
	}
}

func (l *Logic) fillLiked(ctx context.Context, list []InfoRes, userID uint64) error {
	if len(list) == 0 {
		return nil
	}
	videoIDs := make([]uint64, len(list))
	for i, item := range list {
		videoIDs[i] = item.Id
	}
	liked, err := l.likes.FilterLiked(ctx, likeevent.TargetVideo, userID, videoIDs)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].IsLiked = liked[list[i].Id]
	}
	return nil
}

func (l *Logic) fillFollowed(ctx context.Context, list []InfoRes, userID uint64) error {
	authorIDs := make([]uint64, 0, len(list))
	for _, item := range list {
		if item.AuthorID == 0 || item.AuthorID == userID {
			continue
		}
		authorIDs = append(authorIDs, item.AuthorID)
	}
	if len(authorIDs) == 0 {
		return nil
	}
	followed, err := l.follows.FilterFollowing(ctx, userID, authorIDs)
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].AuthorID == 0 || list[i].AuthorID == userID {
			list[i].IsFollow = false
			continue
		}
		list[i].IsFollow = followed[list[i].AuthorID]
	}
	return nil
}

func (l *Logic) getInfoWithCache(ctx context.Context, videoID uint64) (InfoRes, bool, error) {
	cache, err := l.videos.GetInfoCache(ctx, videoID)
	if err != nil {
		return InfoRes{}, false, err
	}
	if cache == nil {
		return l.rebuildInfoCacheOnMiss(ctx, videoID)
	}
	if cache.ExpireAt <= time.Now().Unix() {
		l.tryRefreshInfoCacheAsync(videoID)
	}
	if cache.Empty {
		return InfoRes{}, false, nil
	}
	if cache.Entry == nil {
		return l.rebuildInfoCacheOnMiss(ctx, videoID)
	}
	return l.fromCacheEntry(*cache.Entry), true, nil
}

func (l *Logic) rebuildInfoCacheOnMiss(ctx context.Context, videoID uint64) (InfoRes, bool, error) {
	for i := 0; i < infoMissRetryTimes; i++ {
		token, locked, err := l.videos.TryLockRebuild(ctx, videoID, infoRebuildLockTTL)
		if err != nil {
			return InfoRes{}, false, err
		}
		if locked {
			defer func() { _ = l.videos.UnlockRebuild(ctx, videoID, token) }()
			return l.loadInfoFromDBAndWriteCache(ctx, videoID)
		}

		time.Sleep(infoMissRetrySleep)
		cache, getErr := l.videos.GetInfoCache(ctx, videoID)
		if getErr != nil {
			return InfoRes{}, false, getErr
		}
		if cache == nil {
			continue
		}
		if cache.Empty {
			return InfoRes{}, false, nil
		}
		if cache.Entry != nil {
			return l.fromCacheEntry(*cache.Entry), true, nil
		}
	}
	return l.loadInfoFromDBAndWriteCache(ctx, videoID)
}

func (l *Logic) tryRefreshInfoCacheAsync(videoID uint64) {
	ctx := context.Background()
	token, locked, err := l.videos.TryLockRebuild(ctx, videoID, infoRebuildLockTTL)
	if err != nil || !locked {
		return
	}
	go func() {
		defer func() { _ = l.videos.UnlockRebuild(ctx, videoID, token) }()
		_, _, _ = l.loadInfoFromDBAndWriteCache(ctx, videoID)
	}()
}

func (l *Logic) loadInfoFromDBAndWriteCache(ctx context.Context, videoID uint64) (InfoRes, bool, error) {
	item, err := l.videos.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			setErr := l.videos.SetInfoCache(ctx, videoID, videorepo.InfoCache{
				Empty:    true,
				ExpireAt: time.Now().Add(infoNullLogicalTTL).Unix(),
			})
			if setErr != nil {
				return InfoRes{}, false, setErr
			}
			return InfoRes{}, false, nil
		}
		return InfoRes{}, false, err
	}
	info := l.toInfoRes(*item)
	entry := toCacheEntry(info)
	if err = l.videos.SetInfoCache(ctx, videoID, videorepo.InfoCache{
		Entry:    &entry,
		ExpireAt: time.Now().Add(infoLogicalTTL).Unix(),
	}); err != nil {
		return InfoRes{}, false, err
	}
	return info, true, nil
}

func (l *Logic) fromCacheEntry(entry videorepo.InfoCacheEntry) InfoRes {
	return InfoRes{
		Id:           entry.ID,
		AuthorID:     entry.AuthorID,
		AuthorName:   entry.AuthorName,
		AuthorAvatar: entry.AuthorAvatar,
		Title:        entry.Title,
		Description:  entry.Description,
		CoverURL:     entry.CoverURL,
		PlayURL:      entry.PlayURL,
		CommentCount: entry.CommentCount,
		LikeCount:    entry.LikeCount,
		Status:       entry.Status,
		CreatedAt:    entry.CreatedAt,
	}
}

func toCacheEntry(info InfoRes) videorepo.InfoCacheEntry {
	return videorepo.InfoCacheEntry{
		ID:           info.Id,
		AuthorID:     info.AuthorID,
		AuthorName:   info.AuthorName,
		AuthorAvatar: info.AuthorAvatar,
		Title:        info.Title,
		Description:  info.Description,
		CoverURL:     info.CoverURL,
		PlayURL:      info.PlayURL,
		LikeCount:    info.LikeCount,
		CommentCount: info.CommentCount,
		Status:       info.Status,
		CreatedAt:    info.CreatedAt,
	}
}

// HandleUserUpdated 订阅用户资料变更，刷新自己表里冗余的作者展示字段
func (l *Logic) HandleUserUpdated(ctx context.Context, payload []byte) error {
	var updated userevent.UpdatedEvent
	if err := json.Unmarshal(payload, &updated); err != nil {
		return consumer.Permanent(err)
	}
	if updated.UserID == 0 {
		return consumer.Permanent(errors.New("用户资料事件里没有 userId"))
	}
	if err := l.videos.UpdateAuthorInfo(ctx, updated.UserID, updated.Nickname, updated.AvatarURL); err != nil {
		slog.Error("刷新视频作者信息失败", "author_id", updated.UserID, "error", err)
		return err
	}
	return nil
}
