package upload

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/platform/httpx"
	"strings"
	"time"
)

type SourceType string

const (
	Avatar SourceType = "avatar"
	Cover  SourceType = "cover"
	Video  SourceType = "video"
)

var (
	imageExt = map[string]struct{}{
		".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {},
	}
	videoExt = map[string]struct{}{
		".mp4": {}, ".mov": {}, ".avi": {}, ".mkv": {},
	}
)

// UploadFile 上传文件，只需 file + 类型
// 返回数据库要存的相对路径，例如 /avatar/xxx.webp
func UploadFile(file *multipart.FileHeader, sourceType SourceType) (string, error) {
	subDir, allowExt, err := getConfig(sourceType)
	if err != nil {
		return "", err
	}

	// 校验后缀
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if _, ok := allowExt[ext]; !ok {
		return "", httpx.New(httpx.CodeBadRequest, "文件类型不支持")
	}

	// 生成唯一文件名
	fileName, err := BuildUniqueFileName(ext)
	if err != nil {
		return "", err
	}

	// 构造目录
	dir := filepath.Join(constants.StoragePath, strings.Trim(subDir, "/"))
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	dst := filepath.Join(dir, fileName)

	// 写文件
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err = io.Copy(out, src); err != nil {
		return "", err
	}

	return "/" + filepath.ToSlash(filepath.Join(strings.Trim(subDir, "/"), fileName)), nil
}

// Delete 删除文件，只需 type + 数据库存储路径
func Delete(sourceType SourceType, storedPath string) error {
	if storedPath == "" {
		return nil
	}

	_, err := getSubDir(sourceType)
	if err != nil {
		return err
	}

	// 确保路径干净
	relativePath := strings.TrimLeft(storedPath, "/\\")
	relativePath = filepath.FromSlash(relativePath)

	absPath := filepath.Join(constants.StoragePath, relativePath)

	absBase, _ := filepath.Abs(constants.StoragePath)
	absTarget, _ := filepath.Abs(absPath)
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") {
		return errors.New("file path escapes storage root")
	}

	if err := os.Remove(absTarget); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// BuildUniqueFileName 生成唯一文件名
func BuildUniqueFileName(ext string) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	normalizedExt := strings.ToLower(ext)
	if normalizedExt != "" && !strings.HasPrefix(normalizedExt, ".") {
		normalizedExt = "." + normalizedExt
	}

	return fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(raw), normalizedExt), nil
}

// getConfig 根据 sourceType 返回子目录和允许后缀
func getConfig(t SourceType) (string, map[string]struct{}, error) {
	switch t {
	case Avatar:
		return constants.AvatarPrefix, imageExt, nil
	case Cover:
		return constants.CoverPrefix, imageExt, nil
	case Video:
		return constants.VideoPrefix, videoExt, nil
	default:
		return "", nil, errors.New("invalid source type")
	}
}

// getSubDir 仅返回子目录
func getSubDir(sourceType SourceType) (string, error) {
	switch sourceType {
	case Avatar:
		return constants.AvatarPrefix, nil
	case Cover:
		return constants.CoverPrefix, nil
	case Video:
		return constants.VideoPrefix, nil
	default:
		return "", errors.New("invalid source type")
	}
}
