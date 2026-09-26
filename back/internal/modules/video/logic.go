package video

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	commentevent "simple_tiktok/internal/modules/comment/event"
	followrepo "simple_tiktok/internal/modules/follow/repo"
	likeevent "simple_tiktok/internal/modules/like/event"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userevent "simple_tiktok/internal/modules/user/event"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	videorepo "simple_tiktok/internal/modules/video/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/platform/upload"
)

const (
	infoLogicalTTL     = 5 * time.Minute
	infoNullLogicalTTL = 2 * time.Minute
	infoRebuildLockTTL = 10 * time.Second
	infoMissRetryTimes = 8
	infoMissRetrySleep = 30 * time.Millisecond
)

// Logic 视频模块的业务逻辑
type Logic struct {
	videos   *videorepo.Repo
	users    *userrepo.Repo
	likes    *likerepo.Repo
	follows  *followrepo.Repo
	producer *producer.Producer
	uploader *upload.Uploader
}

func (l *Logic) CreateVideo(ctx context.Context, createReq CreateReq, userID uint64) (CreateRes, error) {
	// 作者展示字段来自 user 表，不取令牌里的昵称，避免改昵称之后显示成旧值
	user, err := l.users.GetByID(ctx, userID)
	if err != nil {
		return CreateRes{}, err
	}

	coverPath, err := l.uploader.Save(createReq.Cover, upload.Cover)
	if err != nil {
		return CreateRes{}, err
	}
	playPath, err := l.uploader.Save(createReq.Play, upload.Video)
	if err != nil {
		if deleteErr := l.uploader.Delete(upload.Cover, coverPath); deleteErr != nil {
			return CreateRes{}, deleteErr
		}
		return CreateRes{}, err
	}

	item := videorepo.Video{
		Title:        createReq.Title,
		Description:  createReq.Description,
		AuthorID:     userID,
		AuthorName:   user.NickName,
		AuthorAvatar: user.AvatarURL,
		PlayURL:      playPath,
		CoverURL:     coverPath,
	}
	if err = l.videos.Create(ctx, &item); err != nil {
		if deleteErr := l.uploader.Delete(upload.Video, playPath); deleteErr != nil {
			return CreateRes{}, deleteErr
		}
		return CreateRes{}, err
	}

	// created_at 由数据库生成，Create 后重新查询拿真实时间再发事件
	created, err := l.videos.GetByID(ctx, item.ID)
	if err != nil {
		return CreateRes{}, err
	}

	// 通知 feed 模块把新视频加进索引
	if err = l.producer.Publish(ctx, topic.VideoCreated, strconv.FormatUint(created.ID, 10), videoevent.CreatedEvent{
		VideoID:   created.ID,
		AuthorID:  created.AuthorID,
		CreatedAt: created.CreateTime,
	}); err != nil {
		return CreateRes{}, err
	}

	return CreateRes{Id: created.ID, Url: l.uploader.URL(created.PlayURL)}, nil
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

	// video 自己删物理文件，comment 删评论，feed 清索引
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

	var err error
	if switched.Liked {
		err = l.videos.IncreaseLikeCount(ctx, switched.TargetID)
	} else {
		err = l.videos.DecreaseLikeCount(ctx, switched.TargetID)
	}
	if err != nil {
		slog.Error("更新视频点赞数失败", "video_id", switched.TargetID, "error", err)
		return err
	}
	return l.videos.DeleteInfoCache(ctx, switched.TargetID)
}

// HandleCommentCreated 订阅评论创建事件，加评论数
func (l *Logic) HandleCommentCreated(ctx context.Context, payload []byte) error {
	var created commentevent.CreatedEvent
	if err := json.Unmarshal(payload, &created); err != nil {
		return consumer.Permanent(err)
	}
	if created.VideoID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有 videoId"))
	}
	if err := l.videos.IncreaseCommentCount(ctx, created.VideoID); err != nil {
		slog.Error("更新视频评论数失败", "video_id", created.VideoID, "error", err)
		return err
	}
	return l.videos.DeleteInfoCache(ctx, created.VideoID)
}

// HandleCommentDeleted 订阅评论删除事件，减评论数
func (l *Logic) HandleCommentDeleted(ctx context.Context, payload []byte) error {
	var deleted commentevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("评论事件里没有 videoId"))
	}
	if err := l.videos.DecreaseCommentCount(ctx, deleted.VideoID); err != nil {
		slog.Error("更新视频评论数失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	return l.videos.DeleteInfoCache(ctx, deleted.VideoID)
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
