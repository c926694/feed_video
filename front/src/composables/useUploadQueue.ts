import { ref } from "vue";
import OSS from "ali-oss";
import { createVideo, fetchUploadCredential, updateVideoStatus } from "@/api";
import type { UploadCredential } from "@/api/video";

// 封面：小于 5MB 走一次直传，超过 10MB 拒绝；视频：小于 100MB 走一次直传，超过 100MB 走分片，上限 10GB
export const COVER_DIRECT_LIMIT = 5 * 1024 * 1024;
export const COVER_MAX_SIZE = 10 * 1024 * 1024;
export const VIDEO_DIRECT_LIMIT = 100 * 1024 * 1024;
export const VIDEO_MAX_SIZE = 10 * 1024 * 1024 * 1024;

export interface UploadTask {
  videoId: number;
  title: string;
  coverFile: File;
  playFile: File;
  coverLocalUrl: string;
  credential: UploadCredential;
  progress: number;
  status: "uploading" | "failed";
}

// 模块级单例，页面跳转不会中断上传
const tasks = ref<UploadTask[]>([]);

function extOf(name: string) {
  const index = name.lastIndexOf(".");
  return index >= 0 ? name.slice(index) : "";
}

function buildClient(credential: UploadCredential) {
  return new OSS({
    region: credential.region,
    bucket: credential.bucket,
    accessKeyId: credential.accessKeyId,
    accessKeySecret: credential.accessKeySecret,
    stsToken: credential.securityToken
  });
}

// uploadObject 按大小分流：小于 multipartLimit 走一次直传，否则分片加 checkpoint 断点续传
async function uploadObject(
  client: OSS,
  key: string,
  file: File,
  onProgress: (percent: number) => void,
  multipartLimit: number
) {
  if (file.size < multipartLimit) {
    await client.put(key, file);
    onProgress(100);
    return;
  }
  const checkpointKey = `feed-oss-checkpoint:${key}`;
  const checkpoint: { file: File; name: string; content: Record<string, unknown> } = {
    file,
    name: key,
    content: {}
  };
  try {
    const saved = localStorage.getItem(checkpointKey);
    if (saved) {
      checkpoint.content = JSON.parse(saved) as Record<string, unknown>;
    }
  } catch {
    // 断点记录损坏就重新上传
  }
  try {
    await client.multipartUpload(key, file, {
      partSize: 1024 * 1024 * 10,
      progress: (p: number) => onProgress(Math.round(p * 100)),
      checkpoint
    });
    localStorage.removeItem(checkpointKey);
  } catch (error) {
    try {
      localStorage.setItem(checkpointKey, JSON.stringify(checkpoint.content));
    } catch {
      // 断点记录写不进本地存储不影响上传错误
    }
    throw error;
  }
}

async function runUpload(task: UploadTask) {
  try {
    task.status = "uploading";
    task.progress = 0;
    const client = buildClient(task.credential);
    await uploadObject(
      client,
      task.credential.coverKey,
      task.coverFile,
      (p) => {
        task.progress = Math.round(p * 0.3);
      },
      COVER_DIRECT_LIMIT
    );
    await uploadObject(
      client,
      task.credential.playKey,
      task.playFile,
      (p) => {
        task.progress = Math.round(30 + p * 0.7);
      },
      VIDEO_DIRECT_LIMIT
    );
    await updateVideoStatus(task.videoId, "published");
    removeTask(task.videoId);
  } catch {
    task.status = "failed";
    task.progress = 0;
    await updateVideoStatus(task.videoId, "failed").catch(() => undefined);
  }
}

// startUpload 拿凭证、创建发布记录、入队并启动后台直传，返回 videoId
export async function startUpload(input: { title: string; description: string; coverFile: File; playFile: File }): Promise<number> {
  if (input.coverFile.size > COVER_MAX_SIZE) {
    throw new Error("封面不能超过 10MB");
  }
  if (input.playFile.size > VIDEO_MAX_SIZE) {
    throw new Error("视频不能超过 10GB");
  }
  const credential = await fetchUploadCredential({
    coverExt: extOf(input.coverFile.name),
    playExt: extOf(input.playFile.name)
  });
  const created = await createVideo({
    title: input.title,
    description: input.description,
    coverKey: credential.coverKey,
    playKey: credential.playKey
  });
  const task: UploadTask = {
    videoId: created.id,
    title: input.title,
    coverFile: input.coverFile,
    playFile: input.playFile,
    coverLocalUrl: URL.createObjectURL(input.coverFile),
    credential,
    progress: 0,
    status: "uploading"
  };
  tasks.value.push(task);
  void runUpload(task);
  return created.id;
}

// retryTask 上传失败后重试：重新拿凭证（沿用视频原有的存储路径），重新上传
export async function retryTask(videoId: number) {
  const task = tasks.value.find((item) => item.videoId === videoId);
  if (!task) return;
  const credential = await updateVideoStatus(videoId, "created");
  if (!credential) return;
  task.credential = credential;
  await runUpload(task);
}

// removeTask 用户删除发布中或失败的视频后，把任务移出队列
export function removeTask(videoId: number) {
  const index = tasks.value.findIndex((item) => item.videoId === videoId);
  if (index >= 0) {
    URL.revokeObjectURL(tasks.value[index].coverLocalUrl);
    tasks.value.splice(index, 1);
  }
}

export function useUploadQueue() {
  return { tasks };
}
