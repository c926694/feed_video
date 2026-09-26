package comment

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"gorm.io/gorm"

	commentevent "simple_tiktok/internal/modules/comment/event"
	commentrepo "simple_tiktok/internal/modules/comment/repo"
	likeevent "simple_tiktok/internal/modules/like/event"
	likerepo "simple_tiktok/internal/modules/like/repo"
	userevent "simple_tiktok/internal/modules/user/event"
	userrepo "simple_tiktok/internal/modules/user/repo"
	videoevent "simple_tiktok/internal/modules/video/event"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
	"simple_tiktok/internal/platform/upload"
)

// Logic 评论模块的业务逻辑
type Logic struct {
	comments *commentrepo.Repo
	users    *userrepo.Repo
	likes    *likerepo.Repo
	producer *producer.Producer
	uploader *upload.Uploader
}

func (l *Logic) Create(ctx context.Context, userID uint64, createReq CreateReq) (*InfoRes, error) {
	if createReq.Content == "" {
		return nil, httpx.New(httpx.CodeBadRequest, "评论内容不能为空")
	}
	commenterName := ""
	commenterAvatar := ""
	if user, userErr := l.users.GetByID(ctx, userID); userErr == nil {
		commenterName = user.NickName
		commenterAvatar = user.AvatarURL
	}
	item := commentrepo.Comment{
		Content:         createReq.Content,
		VideoID:         createReq.VideoID,
		Commenter:       userID,
		CommenterName:   commenterName,
		CommenterAvatar: commenterAvatar,
	}
	if err := l.comments.Create(ctx, &item); err != nil {
		return nil, err
	}

	// video 用来加评论数，feed 用来加热度
	if err := l.producer.Publish(ctx, topic.CommentCreated, strconv.FormatUint(item.ID, 10), commentevent.CreatedEvent{
		CommentID: item.ID,
		VideoID:   item.VideoID,
		Commenter: item.Commenter,
	}); err != nil {
		return nil, err
	}

	list := []InfoRes{{
		Id:              item.ID,
		VideoId:         item.VideoID,
		Commenter:       item.Commenter,
		Content:         item.Content,
		LikeCount:       item.LikeCount,
		CreatedAt:       item.CreatedAt,
		commenterName:   item.CommenterName,
		commenterAvatar: item.CommenterAvatar,
	}}
	l.fillAuthor(list)
	return &list[0], nil
}

func (l *Logic) Delete(ctx context.Context, userID uint64, commentID uint64) error {
	item, err := l.comments.GetByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.New(httpx.CodeNotFound, "评论不存在")
		}
		return err
	}
	if item.Commenter != userID {
		return httpx.New(httpx.CodeForbidden, "只能删除自己的评论")
	}
	if err = l.comments.Delete(ctx, commentID); err != nil {
		return err
	}
	return l.producer.Publish(ctx, topic.CommentDeleted, strconv.FormatUint(commentID, 10), commentevent.DeletedEvent{
		CommentID: commentID,
		VideoID:   item.VideoID,
	})
}

func (l *Logic) ListByVideo(ctx context.Context, videoID uint64, userID uint64) ([]InfoRes, error) {
	items, err := l.comments.ListByVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}
	list := make([]InfoRes, 0, len(items))
	for _, item := range items {
		list = append(list, InfoRes{
			Id:              item.ID,
			VideoId:         item.VideoID,
			Commenter:       item.Commenter,
			Content:         item.Content,
			LikeCount:       item.LikeCount,
			CreatedAt:       item.CreatedAt,
			commenterName:   item.CommenterName,
			commenterAvatar: item.CommenterAvatar,
		})
	}
	l.fillAuthor(list)
	if err = l.fillLiked(ctx, list, userID); err != nil {
		return nil, err
	}
	return list, nil
}

// HandleLikeSwitched 订阅点赞事件，只处理评论点赞
func (l *Logic) HandleLikeSwitched(ctx context.Context, payload []byte) error {
	var switched likeevent.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Target != likeevent.TargetComment {
		return nil
	}
	if switched.TargetID == 0 {
		return consumer.Permanent(errors.New("点赞事件里没有 targetId"))
	}

	var err error
	if switched.Liked {
		err = l.comments.IncreaseLikeCount(ctx, switched.TargetID)
	} else {
		err = l.comments.DecreaseLikeCount(ctx, switched.TargetID)
	}
	if err != nil {
		slog.Error("更新评论点赞数失败", "comment_id", switched.TargetID, "error", err)
		return err
	}
	return nil
}

// HandleVideoDeleted 视频被删除后清理它下面的评论
func (l *Logic) HandleVideoDeleted(ctx context.Context, payload []byte) error {
	var deleted videoevent.DeletedEvent
	if err := json.Unmarshal(payload, &deleted); err != nil {
		return consumer.Permanent(err)
	}
	if deleted.VideoID == 0 {
		return consumer.Permanent(errors.New("删除视频事件里没有 videoId"))
	}
	if err := l.comments.DeleteByVideo(ctx, deleted.VideoID); err != nil {
		slog.Error("删除视频评论失败", "video_id", deleted.VideoID, "error", err)
		return err
	}
	return nil
}

// fillAuthor 用评论行上冗余的字段拼出作者信息，不再读 user 表
func (l *Logic) fillAuthor(list []InfoRes) {
	for i := range list {
		list[i].Author = AuthorRes{
			UserID:    list[i].Commenter,
			Nickname:  list[i].commenterName,
			AvatarURL: l.uploader.URL(list[i].commenterAvatar),
		}
	}
}

// HandleUserUpdated 订阅用户资料变更，刷新自己表里冗余的评论者展示字段
func (l *Logic) HandleUserUpdated(ctx context.Context, payload []byte) error {
	var updated userevent.UpdatedEvent
	if err := json.Unmarshal(payload, &updated); err != nil {
		return consumer.Permanent(err)
	}
	if updated.UserID == 0 {
		return consumer.Permanent(errors.New("用户资料事件里没有 userId"))
	}
	if err := l.comments.UpdateCommenterInfo(ctx, updated.UserID, updated.Nickname, updated.AvatarURL); err != nil {
		slog.Error("刷新评论者信息失败", "commenter", updated.UserID, "error", err)
		return err
	}
	return nil
}

func (l *Logic) fillLiked(ctx context.Context, list []InfoRes, userID uint64) error {
	if len(list) == 0 {
		return nil
	}
	commentIDs := make([]uint64, len(list))
	for i, item := range list {
		commentIDs[i] = item.Id
	}
	liked, err := l.likes.FilterLiked(ctx, likeevent.TargetComment, userID, commentIDs)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].IsLiked = liked[list[i].Id]
	}
	return nil
}
