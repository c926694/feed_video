package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"simple_tiktok/internal/platform/config"
	"simple_tiktok/internal/platform/httpx"
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

// Uploader 把上传文件写入存储目录，并把存储路径转换成对外访问地址
type Uploader struct {
	basePath  string
	urlPrefix string
	dirs      map[SourceType]string
}

// New 创建 Uploader，并在启动阶段建好目录
func New(cfg config.UploadConfig) (*Uploader, error) {
	dirs := map[SourceType]string{
		Avatar: strings.Trim(cfg.AvatarDir, "/"),
		Cover:  strings.Trim(cfg.CoverDir, "/"),
		Video:  strings.Trim(cfg.VideoDir, "/"),
	}
	u := &Uploader{
		basePath:  cfg.BasePath,
		urlPrefix: strings.TrimRight(cfg.URLPrefix, "/"),
		dirs:      dirs,
	}
	for source, dir := range dirs {
		if dir == "" {
			return nil, fmt.Errorf("%s 的存储目录没有配置", source)
		}
		if err := os.MkdirAll(filepath.Join(u.basePath, dir), 0o755); err != nil {
			return nil, fmt.Errorf("创建 %s 目录失败: %w", dir, err)
		}
	}
	return u, nil
}

// Save 保存上传文件，返回数据库要存的相对路径，例如 /avatar/xxx.webp
func (u *Uploader) Save(file *multipart.FileHeader, sourceType SourceType) (string, error) {
	dir, allowExt, err := u.locate(sourceType)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if _, ok := allowExt[ext]; !ok {
		return "", httpx.New(httpx.CodeBadRequest, "文件类型不支持")
	}

	fileName, err := BuildUniqueFileName(ext)
	if err != nil {
		return "", err
	}
	target := filepath.Join(u.basePath, dir, fileName)

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err = io.Copy(out, src); err != nil {
		return "", err
	}

	return "/" + filepath.ToSlash(filepath.Join(dir, fileName)), nil
}

// Delete 删除存储目录里的文件，文件不存在时视为删除成功
func (u *Uploader) Delete(sourceType SourceType, storedPath string) error {
	if storedPath == "" {
		return nil
	}
	dir, _, err := u.locate(sourceType)
	if err != nil {
		return err
	}

	relative := filepath.ToSlash(strings.TrimLeft(storedPath, "/\\"))
	if !strings.HasPrefix(relative, dir+"/") {
		return fmt.Errorf("文件路径 %s 不在 %s 目录内", storedPath, dir)
	}

	if err := os.Remove(filepath.Join(u.basePath, filepath.FromSlash(relative))); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// URL 把数据库里的存储路径转换成对外访问地址
func (u *Uploader) URL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return u.urlPrefix + path
	}
	return u.urlPrefix + "/" + path
}

// BuildUniqueFileName 生成唯一文件名
func BuildUniqueFileName(ext string) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	normalized := strings.ToLower(ext)
	if normalized != "" && !strings.HasPrefix(normalized, ".") {
		normalized = "." + normalized
	}
	return fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(raw), normalized), nil
}

func (u *Uploader) locate(sourceType SourceType) (string, map[string]struct{}, error) {
	dir, ok := u.dirs[sourceType]
	if !ok {
		return "", nil, fmt.Errorf("不支持的上传类型 %s", sourceType)
	}
	if sourceType == Video {
		return dir, videoExt, nil
	}
	return dir, imageExt, nil
}
