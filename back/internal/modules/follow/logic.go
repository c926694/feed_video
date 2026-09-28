package follow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"simple_tiktok/internal/modules/follow/event"
	"simple_tiktok/internal/modules/follow/repo"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka/consumer"
	"simple_tiktok/internal/platform/kafka/producer"
	"simple_tiktok/internal/platform/kafka/topic"
)

// Logic 关注模块的业务逻辑。关注关系先写 Redis，关系表由订阅方维护。
type Logic struct {
	repo     *repo.Repo
	producer *producer.Producer
}

// SetFollow 把关注状态设置成目标态，幂等：重复设置同一目标态不发事件
func (l *Logic) SetFollow(ctx context.Context, targetUserID uint64, currentUserID uint64, active bool) (bool, error) {
	if targetUserID == currentUserID {
		return false, httpx.New(httpx.CodeBadRequest, "不能关注自己")
	}
	exists, err := l.repo.UserExists(ctx, targetUserID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, httpx.New(httpx.CodeNotFound, "用户不存在")
	}
	return l.setFollow(ctx, targetUserID, currentUserID, active)
}

// setFollow 把关注状态设置成目标态。状态没变时幂等返回，不发事件；
// 状态变了才发事件，发布失败时把状态写回之前的状态
func (l *Logic) setFollow(ctx context.Context, targetUserID uint64, currentUserID uint64, active bool) (bool, error) {
	changed, err := l.repo.Set(ctx, currentUserID, targetUserID, active)
	if err != nil {
		return false, err
	}
	if !changed {
		// 本来就是目标态，幂等返回，不重复发事件
		return active, nil
	}

	if err = l.producer.Publish(ctx, topic.FollowSwitched, strconv.FormatUint(targetUserID, 10), event.SwitchedEvent{
		Follower:  currentUserID,
		Following: targetUserID,
		Followed:  active,
	}); err != nil {
		if _, rollbackErr := l.repo.Set(ctx, currentUserID, targetUserID, !active); rollbackErr != nil {
			return false, httpx.New(httpx.CodeInternal, fmt.Sprintf("关注状态回滚失败: %v", rollbackErr))
		}
		return false, err
	}
	return active, nil
}

// HandleSwitched 订阅自己的事件，维护 follow 表
func (l *Logic) HandleSwitched(ctx context.Context, payload []byte) error {
	var switched event.SwitchedEvent
	if err := json.Unmarshal(payload, &switched); err != nil {
		return consumer.Permanent(err)
	}
	if switched.Follower == 0 || switched.Following == 0 {
		return consumer.Permanent(errors.New("关注事件里缺少用户 ID"))
	}

	var err error
	if switched.Followed {
		err = l.repo.Create(ctx, switched.Follower, switched.Following)
	} else {
		err = l.repo.Delete(ctx, switched.Follower, switched.Following)
	}
	if err != nil {
		slog.Error("维护关注关系失败", "follower", switched.Follower, "following", switched.Following, "error", err)
		return err
	}
	return nil
}
