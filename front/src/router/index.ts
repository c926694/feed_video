import { createRouter, createWebHistory } from "vue-router";
import FeedPage from "@/views/FeedPage.vue";
import LoginPage from "@/views/LoginPage.vue";
import MessagePage from "@/views/MessagePage.vue";
import ProfilePage from "@/views/ProfilePage.vue";
import ProfileVideoPage from "@/views/ProfileVideoPage.vue";
import RegisterPage from "@/views/RegisterPage.vue";
import UploadPage from "@/views/UploadPage.vue";
import UserProfilePage from "@/views/UserProfilePage.vue";
import VideoPage from "@/views/VideoPage.vue";
import { getToken } from "@/utils/storage";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", redirect: "/feed" },
    { path: "/login", component: LoginPage, meta: { public: true } },
    { path: "/register", component: RegisterPage, meta: { public: true } },
    { path: "/feed", component: FeedPage },
    { path: "/messages", component: MessagePage },
    { path: "/upload", component: UploadPage },
    { path: "/video/:id", component: VideoPage },
    { path: "/profile", component: ProfilePage },
    { path: "/profile/videos", component: ProfileVideoPage },
    { path: "/profile/:id", component: UserProfilePage },
    { path: "/profile/:id/videos", component: ProfileVideoPage }
  ]
});

router.beforeEach((to) => {
  if (to.meta.public) return true;
  if (getToken()) return true;
  return "/login";
});

export default router;
