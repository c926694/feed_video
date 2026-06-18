package video

import (
	"context"
	"encoding/json"
	"log"
	"simple_tiktok/internal/modulekit"
	"simple_tiktok/internal/mq/event"
	consumer2 "simple_tiktok/internal/mq/kafka/consumer"
	"simple_tiktok/internal/pkg/upload"
	"simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/svc"

	"github.com/segmentio/kafka-go"
)

func RegisterConsumers(registrar modulekit.ConsumerRegistrar, ctx *svc.ServiceContext) error {
	videoRepo := mysql.NewVideoRepo(ctx.DB)

	deleteVideoConsumer := consumer2.NewConsumer(ctx.KafkaBrokers, event.DeleteVideoTopic, "video-delete-group")

	registrar.Add("video.delete", func() error {
		return deleteVideoConsumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
			return handleDeleteVideo(msg, videoRepo)
		})
	})
	return nil
}

func handleDeleteVideo(msg kafka.Message, videoRepo *mysql.VideoRepo) error {
	var deleteVideoEvent event.DeleteVideoEvent
	if err := json.Unmarshal(msg.Value, &deleteVideoEvent); err != nil {
		log.Println(err)
		return err
	}
	err := upload.Delete(upload.Video, deleteVideoEvent.PlayURL)
	if err != nil {
		log.Println(err)
		return err
	}
	err = upload.Delete(upload.Cover, deleteVideoEvent.CoverURL)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
