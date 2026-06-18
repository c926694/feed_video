package follow

import (
	"context"
	"encoding/json"
	"log"
	"simple_tiktok/internal/model"
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	consumer2 "simple_tiktok/internal/mq/kafka/consumer"
	"simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/svc"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	followRepo := mysql.NewFollowRepo(ctx.DB)
	userRepo := mysql.NewUserRepo(ctx.DB)

	followConsumer := consumer2.NewConsumer(ctx.KafkaBrokers, event.FollowTopic, "follow-group")

	registrar.Add("follow.switch", func() error {
		return followConsumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
			return handleFollow(msg, followRepo, userRepo)
		})
	})
	return nil
}

func handleFollow(msg kafka.Message, followRepo *mysql.FollowRepo, userRepo *mysql.UserRepo) error {
	var followEvent event.FollowEvent
	if err := json.Unmarshal(msg.Value, &followEvent); err != nil {
		log.Println(err)
		return err
	}

	follow := &model.Follow{Following: followEvent.Following, Follower: followEvent.Follower}
	tx := followRepo.DB().Begin()
	var err error
	switch followEvent.EventType {
	case event.Follow:
		err = tx.Create(follow).Error
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Follower).Update("follow_count", gorm.Expr("follow_count + 1")).Error
		}
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Following).Update("follower_count", gorm.Expr("follower_count + 1")).Error
		}
	case event.Unfollow:
		err = tx.Where("follower = ? and following = ?", followEvent.Follower, followEvent.Following).Delete(&model.Follow{}).Error
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Follower).Update("follow_count", gorm.Expr("CASE WHEN follow_count > 0 THEN follow_count - 1 ELSE 0 END")).Error
		}
		if err == nil {
			err = tx.Model(&model.User{}).Where("id = ?", followEvent.Following).Update("follower_count", gorm.Expr("CASE WHEN follower_count > 0 THEN follower_count - 1 ELSE 0 END")).Error
		}
	default:
		_ = tx.Rollback()
		log.Printf("unsupported follow event type: %s", followEvent.EventType)
		return nil
	}

	if err != nil {
		_ = tx.Rollback()
		log.Println(err)
		return err
	}
	if err = tx.Commit().Error; err != nil {
		log.Println(err)
		return err
	}
	return nil
}
