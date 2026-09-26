package like

import (
	"context"
	"fmt"
	"strconv"

	"simple_tiktok/internal/modules/like/event"
	"simple_tiktok/internal/modules/like/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
)

// Logic 点赞模块的业务逻辑
type Logic struct {
	repo     *repo.Repo
	producer *producer.Producer
}

func (l *Logic) SwitchVideoLike(ctx context.Context, videoID uint64, userID uint64) (bool, error) {
	return l.switchLike(ctx, event.TargetVideo, videoID, userID)
}

func (l *Logic) SwitchCommentLike(ctx context.Context, commentID uint64, userID uint64) (bool, error) {
	return l.switchLike(ctx, event.TargetComment, commentID, userID)
}

func (l *Logic) switchLike(ctx context.Context, target string, targetID uint64, userID uint64) (bool, error) {
	liked, err := l.repo.Switch(ctx, target, targetID, userID)
	if err != nil {
		return false, err
	}

	err = l.producer.Publish(ctx, topic.LikeSwitched, strconv.FormatUint(targetID, 10), event.SwitchedEvent{
		Target:   target,
		TargetID: targetID,
		Liked:    liked,
		Operator: userID,
	})
	if err != nil {
		if rollbackErr := l.repo.Reset(ctx, target, targetID, userID, !liked); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, fmt.Sprintf("点赞状态回滚失败: %v", rollbackErr))
		}
		return false, err
	}
	return liked, nil
}
