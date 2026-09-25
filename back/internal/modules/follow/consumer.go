package follow

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"gorm.io/gorm"

	"simple_tiktok/internal/model"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/platform/kafka"
	"simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/svc"
)

func RegisterConsumers(sub *kafka.Subscriber, ctx *svc.ServiceContext) error {
	followRepo := mysql.NewFollowRepo(ctx.DB)
	sub.Subscribe(kafka.TopicFollow, func(handlerCtx context.Context, payload []byte) error {
		return handleFollow(handlerCtx, payload, followRepo)
	})
	return nil
}

// handleFollow 消费关注事件，在一个事务里更新关注关系与双方的关注计数
func handleFollow(ctx context.Context, payload []byte, followRepo *mysql.FollowRepo) error {
	var followEvent event.FollowEvent
	if err := json.Unmarshal(payload, &followEvent); err != nil {
		return kafka.Permanent(err)
	}
	if followEvent.Follower == 0 || followEvent.Following == 0 {
		return kafka.Permanent(errors.New("关注事件里缺少用户 ID"))
	}

	follow := &model.Follow{Following: followEvent.Following, Follower: followEvent.Follower}
	tx := followRepo.DB().Begin()
	var err error
	switch followEvent.EventType {
	case event.Follow:
		err = tx.Create(follow).Error
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Follower).
				Update("follow_count", gorm.Expr("follow_count + 1")).Error
		}
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Following).
				Update("follower_count", gorm.Expr("follower_count + 1")).Error
		}
	case event.Unfollow:
		err = tx.Where("follower = ? and following = ?", followEvent.Follower, followEvent.Following).
			Delete(&model.Follow{}).Error
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Follower).
				Update("follow_count", gorm.Expr("CASE WHEN follow_count > 0 THEN follow_count - 1 ELSE 0 END")).Error
		}
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Following).
				Update("follower_count", gorm.Expr("CASE WHEN follower_count > 0 THEN follower_count - 1 ELSE 0 END")).Error
		}
	default:
		_ = tx.Rollback()
		slog.Warn("不支持的关注事件类型", "event_type", followEvent.EventType)
		return nil
	}

	if err != nil {
		_ = tx.Rollback()
		slog.Error("处理关注事件失败",
			"follower", followEvent.Follower,
			"following", followEvent.Following,
			"error", err)
		return err
	}
	if err = tx.Commit().Error; err != nil {
		slog.Error("提交关注事务失败", "follower", followEvent.Follower, "following", followEvent.Following, "error", err)
		return err
	}
	return nil
}
