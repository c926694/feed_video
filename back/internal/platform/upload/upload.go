package upload

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

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

// defaultAvatarName 默认头像在 avatar 目录下的文件名，与 user 模块写入的路径一致
const defaultAvatarName = "default.svg"

// defaultAvatarSVG 新用户的默认头像，启动时确保它存在于 bucket 里
const defaultAvatarSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256" fill="none">
  <rect width="256" height="256" rx="128" fill="#1f2937"/>
  <circle cx="128" cy="98" r="46" fill="#9ca3af"/>
  <path d="M56 208c9-35 38-58 72-58s63 23 72 58" fill="#9ca3af"/>
</svg>
`

// Uploader 把上传文件写进阿里云 OSS，并把存储路径转换成对外访问地址。
// 数据库里保存的是不带域名的 object key，例如 /avatar/xxx.webp。
type Uploader struct {
	bucket    *oss.Bucket
	prefix    string
	accessURL string
	dirs      map[SourceType]string
}

// New 创建 Uploader，并在启动阶段确认 bucket 可以访问
func New(cfg config.UploadConfig) (*Uploader, error) {
	if cfg.OSS.Endpoint == "" || cfg.OSS.Bucket == "" {
		return nil, fmt.Errorf("OSS 的 endpoint 与 bucket 必须配置")
	}
	if cfg.OSS.AccessKeyID == "" || cfg.OSS.AccessKeySecret == "" {
		return nil, fmt.Errorf("OSS 的 access key 必须配置")
	}
	for source, dir := range map[SourceType]string{
		Avatar: cfg.AvatarDir,
		Cover:  cfg.CoverDir,
		Video:  cfg.VideoDir,
	} {
		if strings.Trim(dir, "/") == "" {
			return nil, fmt.Errorf("%s 的存储目录没有配置", source)
		}
	}

	client, err := oss.New(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %w", err)
	}
	bucket, err := client.Bucket(cfg.OSS.Bucket)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS bucket 失败: %w", err)
	}
	// 启动时确认 bucket 存在且凭据可用
	if _, err = client.GetBucketInfo(cfg.OSS.Bucket); err != nil {
		return nil, fmt.Errorf("访问 OSS bucket %s 失败: %w", cfg.OSS.Bucket, err)
	}

	host := strings.TrimRight(cfg.OSS.CustomDomain, "/")
	if host == "" {
		host = cfg.OSS.Bucket + "." + cfg.OSS.Endpoint
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}

	uploader := &Uploader{
		bucket:    bucket,
		prefix:    strings.Trim(cfg.OSS.Prefix, "/"),
		accessURL: host,
		dirs: map[SourceType]string{
			Avatar: strings.Trim(cfg.AvatarDir, "/"),
			Cover:  strings.Trim(cfg.CoverDir, "/"),
			Video:  strings.Trim(cfg.VideoDir, "/"),
		},
	}
	if err = uploader.ensureDefaultAvatar(); err != nil {
		return nil, err
	}
	return uploader, nil
}

// ensureDefaultAvatar 确保默认头像存在于 bucket 里，缺失时补一份
func (u *Uploader) ensureDefaultAvatar() error {
	key := u.dirPrefix(u.dirs[Avatar]) + defaultAvatarName
	exists, err := u.bucket.IsObjectExist(key)
	if err != nil {
		return fmt.Errorf("检查默认头像失败: %w", err)
	}
	if exists {
		return nil
	}
	if err = u.bucket.PutObject(key, strings.NewReader(defaultAvatarSVG)); err != nil {
		return fmt.Errorf("上传默认头像 %s 失败: %w", key, err)
	}
	return nil
}

// Save 上传文件，返回数据库要存的相对路径，例如 /avatar/xxx.webp
func (u *Uploader) Save(file *multipart.FileHeader, sourceType SourceType) (string, error) {
	dir, allowExt, err := u.locate(sourceType)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(path.Ext(file.Filename))
	if _, ok := allowExt[ext]; !ok {
		return "", httpx.New(httpx.CodeBadRequest, "文件类型不支持")
	}

	fileName, err := BuildUniqueFileName(ext)
	if err != nil {
		return "", err
	}
	key := path.Join(u.prefix, dir, fileName)

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	if err = u.bucket.PutObject(key, src); err != nil {
		return "", fmt.Errorf("上传 %s 失败: %w", key, err)
	}
	return "/" + key, nil
}

// Delete 删除 bucket 里的对象，对象不存在时视为删除成功
func (u *Uploader) Delete(sourceType SourceType, storedPath string) error {
	if storedPath == "" {
		return nil
	}
	dir, _, err := u.locate(sourceType)
	if err != nil {
		return err
	}

	key := strings.TrimLeft(storedPath, "/")
	if !strings.HasPrefix(key, u.dirPrefix(dir)) {
		return fmt.Errorf("文件路径 %s 不在 %s 目录内", storedPath, dir)
	}

	if err = u.bucket.DeleteObject(key); err != nil {
		if isObjectNotExist(err) {
			return nil
		}
		return fmt.Errorf("删除 %s 失败: %w", key, err)
	}
	return nil
}

// isObjectNotExist 判断 OSS 返回的错误是不是对象不存在
func isObjectNotExist(err error) bool {
	var serviceErr oss.ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr.StatusCode == http.StatusNotFound || serviceErr.Code == "NoSuchKey"
	}
	return false
}

// URL 把数据库里的存储路径转换成对外访问地址
func (u *Uploader) URL(storedPath string) string {
	if storedPath == "" {
		return ""
	}
	if strings.HasPrefix(storedPath, "http://") || strings.HasPrefix(storedPath, "https://") {
		return storedPath
	}
	return u.accessURL + "/" + strings.TrimLeft(storedPath, "/")
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

func (u *Uploader) dirPrefix(dir string) string {
	return path.Join(u.prefix, dir) + "/"
}
