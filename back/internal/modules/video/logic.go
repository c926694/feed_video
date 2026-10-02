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
	"golang.org/x/sync/singleflight"

	favoriteevent "simple_tiktok/internal/modules/favorite/event"
	favoriterepo "simple_tiktok/internal/modules/favorite/repo"
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
	infoLogicalTTL       = 5 * time.Minute        // 详情记录的新鲜度上界
	infoMaxStaleSeconds  = int64(20 * 60)         // 旧值最长可用时间，超过它就不再返回旧值，改为等待一次回源
	infoRefreshTimeout   = 3 * time.Second        // 共享重建的超时
	infoInvalidateMaxIDs = 2000                   // 资料变更时批量清理缓存的视频数上限

	coverMaxSize = 10 << 20 // 封面最大 10MB
	videoMaxSize = 10 << 30 // 视频最大 10GB
)

// Logic 视频模块的业务逻辑
type Logic struct {
	videos    *videorepo.Repo
	users     *userrepo.Repo
	likes     *likerepo.Repo
	favorites *favoriterepo.Repo
	follows   *followrepo.Repo
	producer  *producer.Producer
	uploader  *upload.Uploader
	sts       *sts.Service

	// refreshGroup 详情缓存的共享重建：同一个视频在同一时刻只有一次回源，
	// 其余调用共享它的结果。对外不暴露，只在本模块内使用
	refreshGroup singleflight.Group
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
		if _, _, err := l.transitionStatus(ctx, videoID, userID, videorepo.StatusCreated, videorepo.StatusFailed); err != nil {
			return nil, err
		}
		return nil, nil
	case videorepo.StatusCreated:
		if l.sts == nil {
			return nil, httpx.New(httpx.CodeInternal, "直传服务未配置")
		}
		item, _, err := l.transitionStatus(ctx, videoID, userID, videorepo.StatusFailed, videorepo.StatusCreated)
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
	_, migrated, err := l.transitionStatus(ctx, videoID, userID, videorepo.StatusCreated, videorepo.StatusPublished)
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

// transitionStatus 状态迁移的统一入口：条件更新成功后刷新缓存。
// 返回迁移前的记录和是否真的发生了迁移，迁移幂等
func (l *Logic) transitionStatus(ctx context.Context, videoID uint64, userID uint64, from string, to string) (*videorepo.Video, bool, error) {
	item, err := l.getOwnVideo(ctx, videoID, userID, "只能操作自己的视频")
	if err != nil {
		return nil, false, err
	}
	migrated, err := l.videos.MarkStatus(ctx, videoID, userID, from, to)
	if err != nil {
		return nil, false, err
	}
	if !migrated {
		return item, false, nil
	}
	if err = l.refreshInfoCache(ctx, item, to); err != nil {
		slog.Error("刷新视频详情缓存失败", "video_id", videoID, "status", to, "error", err)
	}
	return item, true, nil
}

// refreshInfoCache 状态迁移后刷新详情缓存：先删保证正确性，
// 只有迁移到 published 才写预热记录，让发布后的第一个读者不用回源
func (l *Logic) refreshInfoCache(ctx context.Context, item *videorepo.Video, status string) error {
	if err := l.videos.DeleteInfoCache(ctx, item.ID); err != nil {
		return err
	}
	if status != videorepo.StatusPublished {
		return nil
	}
	item.Status = status
	entry := toCacheEntry(l.toInfoRes(*item))
	return l.videos.SetInfoCache(ctx, item.ID, videorepo.InfoCache{
		Entry:    &entry,
		ExpireAt: time.Now().Add(infoLogicalTTL).Unix(),
	})
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
	if err = l.fillFavorited(ctx, list, userID); err != nil {
		return nil, err
	}
	if err = l.fillFollowed(ctx, list, userID); err != nil {
		return nil, err
	}
	return list, nil
}

// ListAuthorVideos 取指定作者的已发布视频，双字段游标分页，
// 同时补上当前登录用户对每条视频的点赞、收藏与关注状态
func (l *Logic) ListAuthorVideos(ctx context.Context, authorID uint64, lastCreatedAt int64, lastID uint64, limit uint64, userID uint64) (*AuthorVideosRes, error) {
	if limit == 0 {
		limit = 20
	}
	if limit > 60 {
		limit = 60
	}
	var cursor time.Time
	if lastID > 0 {
		cursor = time.UnixMilli(lastCreatedAt)
	}

	items, err := l.videos.ListByAuthorsBefore(ctx, []uint64{authorID}, limit+1, cursor, lastID)
	if err != nil {
		return nil, err
	}
	hasMore := uint64(len(items)) > limit
	if hasMore {
		items = items[:limit]
	}

	list := make([]InfoRes, 0, len(items))
	for i := range items {
		list = append(list, l.toInfoRes(items[i]))
	}
	if err = l.fillLiked(ctx, list, userID); err != nil {
		return nil, err
	}
	if err = l.fillFavorited(ctx, list, userID); err != nil {
		return nil, err
	}
	if err = l.fillFollowed(ctx, list, userID); err != nil {
		return nil, err
	}

	result := &AuthorVideosRes{List: list, HasMore: hasMore}
	if len(items) > 0 {
		last := items[len(items)-1]
		result.LastCreatedAt = last.CreateTime.UnixMilli()
		result.LastId = last.ID
	}
	return result, nil
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
	if err = l.fillFavorited(ctx, list, userID); err != nil {
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

	// 先删缓存，再删数据库行：缓存删失败直接返回错误，行还在，用户重试安全。
	// 反过来先删行的话，删缓存失败会留下一条仍然可见已删除内容的记录
	if err = l.videos.DeleteInfoCache(ctx, videoID); err != nil {
		return err
	}
	if err = l.videos.Delete(ctx, videoID); err != nil {
		return err
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

// fillFavorited 批量补当前用户对这批视频的收藏状态
func (l *Logic) fillFavorited(ctx context.Context, list []InfoRes, userID uint64) error {
	if len(list) == 0 {
		return nil
	}
	videoIDs := make([]uint64, len(list))
	for i, item := range list {
		videoIDs[i] = item.Id
	}
	favorited, err := l.favorites.FilterFavorited(ctx, userID, videoIDs)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].IsFavorited = favorited[list[i].Id]
	}
	return nil
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
	// 点赞数变了，让详情缓存失效，下一次读重新回源
	return l.videos.DeleteInfoCache(ctx, switched.TargetID)
}

// HandleFavoriteSwitched 订阅收藏事件，按集合大小对账视频收藏数
func (l *Logic) HandleFavoriteSwitched(ctx context.Context, payload []byte) error {
	var switched favoriteevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.VideoID == 0 {
		return consumer.Permanent(errors.New("收藏事件里没有 videoId"))
	}

	count, err := l.favorites.CountFavorites(ctx, switched.VideoID)
	if err != nil {
		slog.Error("统计视频收藏数失败", "video_id", switched.VideoID, "error", err)
		return err
	}
	if err = l.videos.SyncFavoriteCount(ctx, switched.VideoID, count); err != nil {
		slog.Error("对账视频收藏数失败", "video_id", switched.VideoID, "error", err)
		return err
	}
	// 收藏数变了，让详情缓存失效，下一次读重新回源
	return l.videos.DeleteInfoCache(ctx, switched.VideoID)
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
		CommentCount:  item.CommentCount,
		LikeCount:     item.LikeCount,
		FavoriteCount: item.FavoriteCount,
		Status:        item.Status,
		CreatedAt:     item.CreateTime,
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

// getInfoWithCache 读详情缓存：命中返回记录，逻辑过期返回旧值并触发后台共享重建，
// 空值标记返回不存在，键不存在或者旧值太旧时合并等待一次回源
func (l *Logic) getInfoWithCache(ctx context.Context, videoID uint64) (InfoRes, bool, error) {
	record, found, err := l.videos.GetInfoCache(ctx, videoID)
	if err != nil {
		return InfoRes{}, false, err
	}
	if !found {
		// 键不存在，手里没有旧值可以返回，只能等一次合并后的回源
		return l.loadInfoShared(ctx, videoID)
	}
	if record == nil {
		// 空值标记有效期内直接返回不存在
		return InfoRes{}, false, nil
	}
	now := time.Now().Unix()
	if record.ExpireAt <= now {
		if now-record.ExpireAt > infoMaxStaleSeconds {
			// 旧值已经超出最长可用时间，不再返回它，改为等一次回源
			return l.loadInfoShared(ctx, videoID)
		}
		// 逻辑过期：触发后台共享重建，不等待，继续往下返回手里的旧值
		l.triggerInfoRefresh(videoID)
	}
	return l.fromCacheEntry(*record.Entry), true, nil
}

// triggerInfoRefresh 发起共享重建但不读 channel，所以调用方不等待。
// 同一个视频已经有重建在跑时，这次调用共享那一次，不会重复回源
func (l *Logic) triggerInfoRefresh(videoID uint64) {
	_ = l.refreshGroup.DoChan(infoGroupKey(videoID), func() (any, error) {
		refreshCtx, cancel := context.WithTimeout(context.Background(), infoRefreshTimeout)
		defer cancel()
		if _, _, err := l.loadInfoFromDBAndWriteCache(refreshCtx, videoID); err != nil {
			slog.Error("后台刷新视频详情缓存失败", "video_id", videoID, "error", err)
			return nil, err
		}
		return nil, nil
	})
}

// infoResult 共享重建的返回值
type infoResult struct {
	info    InfoRes
	visible bool
}

// loadInfoShared 合并并发回源：同一个视频同一时刻只有一次查询，其余调用共享结果。
// 共享调用用独立 context，调用方断开只放弃等待，那次查询继续跑完并把缓存写好
func (l *Logic) loadInfoShared(ctx context.Context, videoID uint64) (InfoRes, bool, error) {
	channel := l.refreshGroup.DoChan(infoGroupKey(videoID), func() (any, error) {
		refreshCtx, cancel := context.WithTimeout(context.Background(), infoRefreshTimeout)
		defer cancel()
		info, visible, err := l.loadInfoFromDBAndWriteCache(refreshCtx, videoID)
		if err != nil {
			return nil, err
		}
		return infoResult{info: info, visible: visible}, nil
	})

	select {
	case result := <-channel:
		if result.Err != nil {
			return InfoRes{}, false, result.Err
		}
		got, ok := result.Val.(infoResult)
		if !ok {
			return InfoRes{}, false, errors.New("详情缓存重建返回了非预期类型")
		}
		return got.info, got.visible, nil
	case <-ctx.Done():
		return InfoRes{}, false, ctx.Err()
	}
}

// infoGroupKey 共享重建的分组键
func infoGroupKey(videoID uint64) string {
	return fmt.Sprintf("video:info:%d", videoID)
}

// loadInfoFromDBAndWriteCache 重建缓存：读视频行写回详情记录。
// 行不存在写空值标记；未发布的行不写缓存，只返回不可见
func (l *Logic) loadInfoFromDBAndWriteCache(ctx context.Context, videoID uint64) (InfoRes, bool, error) {
	item, err := l.videos.GetByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if setErr := l.videos.SetInfoMiss(ctx, videoID); setErr != nil {
				return InfoRes{}, false, setErr
			}
			return InfoRes{}, false, nil
		}
		return InfoRes{}, false, err
	}
	if item.Status != videorepo.StatusPublished {
		// 未发布的不进缓存，只返回不可见
		return InfoRes{}, false, nil
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
		Id:            entry.ID,
		AuthorID:      entry.AuthorID,
		AuthorName:    entry.AuthorName,
		AuthorAvatar:  entry.AuthorAvatar,
		Title:         entry.Title,
		Description:   entry.Description,
		CoverURL:      entry.CoverURL,
		PlayURL:       entry.PlayURL,
		CommentCount:  entry.CommentCount,
		LikeCount:     entry.LikeCount,
		FavoriteCount: entry.FavoriteCount,
		Status:        entry.Status,
		CreatedAt:     entry.CreatedAt,
	}
}

func toCacheEntry(info InfoRes) videorepo.InfoCacheEntry {
	return videorepo.InfoCacheEntry{
		ID:            info.Id,
		AuthorID:      info.AuthorID,
		AuthorName:    info.AuthorName,
		AuthorAvatar:  info.AuthorAvatar,
		Title:         info.Title,
		Description:   info.Description,
		CoverURL:      info.CoverURL,
		PlayURL:       info.PlayURL,
		LikeCount:     info.LikeCount,
		CommentCount:  info.CommentCount,
		FavoriteCount: info.FavoriteCount,
		Status:        info.Status,
		CreatedAt:     info.CreatedAt,
	}
}

// HandleUserUpdated 订阅用户资料变更，刷新冗余的作者展示字段并清理详情缓存
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

	// 昵称头像存在详情记录里，MySQL 改完之后把这些记录的缓存删掉，
	// 超出上限的部分由逻辑过期兜住
	ids, err := l.videos.ListIDsByAuthor(ctx, updated.UserID, infoInvalidateMaxIDs)
	if err != nil {
		slog.Error("列出作者视频失败", "author_id", updated.UserID, "error", err)
		return err
	}
	if err = l.videos.DeleteInfoCacheBatch(ctx, ids); err != nil {
		slog.Error("批量清理视频详情缓存失败", "author_id", updated.UserID, "error", err)
		return err
	}
	return nil
}
