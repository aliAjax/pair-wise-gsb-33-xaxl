<template>
  <div class="page">
    <el-row :gutter="16">
      <el-col :xs="24" :md="10">
        <el-card>
          <template #header>个人资料</template>
          <el-avatar :size="80" :src="profile?.avatar" class="avatar">{{ profile?.nickname?.[0] }}</el-avatar>
          <el-form label-width="80px" class="profile-form">
            <el-form-item label="用户名"><el-input :model-value="profile?.username" disabled /></el-form-item>
            <el-form-item label="昵称"><el-input v-model="form.nickname" /></el-form-item>
            <el-form-item label="邮箱"><el-input :model-value="profile?.email" disabled /></el-form-item>
            <el-form-item label="简介"><el-input v-model="form.bio" type="textarea" :rows="3" /></el-form-item>
            <el-form-item label="头像"><ImageUploader v-model="form.avatar" /></el-form-item>
            <el-form-item>
              <el-button type="primary" @click="save">保存修改</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="14">
        <el-card class="fav-card">
          <template #header>我的收藏</template>
          <el-table :data="favorites" empty-text="暂无收藏">
            <el-table-column label="类型">
              <template #default="{ row }">{{ FavoriteTargetTypeMap[row.target_type as FavoriteTargetType] }}</template>
            </el-table-column>
            <el-table-column prop="target_id" label="目标 ID" />
            <el-table-column label="时间">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
        <el-card class="draft-card">
          <template #header>
            <div class="card-head">
              <span>养护文章与问答草稿</span>
              <el-radio-group v-model="draftFilter" size="small" @change="loadMyArticles">
                <el-radio-button label="">全部</el-radio-button>
                <el-radio-button label="draft">草稿</el-radio-button>
                <el-radio-button label="invalidated">已失效</el-radio-button>
                <el-radio-button label="published">已发布</el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <el-table :data="myArticles" empty-text="暂无文章或草稿">
            <el-table-column prop="title" label="标题" min-width="200">
              <template #default="{ row }">
                <el-link type="primary" :underline="false" @click="$router.push(`/articles/${row.id}`)">{{ row.title }}</el-link>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="CareArticleStatusTagType[row.status as CareArticleStatus]" size="small">
                  {{ CareArticleStatusMap[row.status as CareArticleStatus] }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="来源" width="100">
              <template #default="{ row }">{{ row.source_question_id ? '问答沉淀' : '自主撰写' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="160">
              <template #default="{ row }">
                <el-button
                  v-if="row.status === 'invalidated' && row.source_question_id"
                  link
                  type="warning"
                  @click="$router.push(`/questions/${row.source_question_id}`)"
                >重新生成</el-button>
                <el-button v-if="row.status === 'draft'" link type="primary" @click="$router.push(`/articles/${row.id}`)">完善/发布</el-button>
                <el-button v-if="row.status === 'published'" link @click="$router.push(`/articles/${row.id}`)">查看</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import ImageUploader from '@/components/common/ImageUploader.vue'
import { useUserStore } from '@/stores/userStore'
import { listFavorites } from '@/api/favorite'
import { listMyArticles } from '@/api/article'
import { FavoriteTargetTypeMap, type Favorite, type FavoriteTargetType } from '@/constants/favorite'
import {
  CareArticleStatusMap,
  CareArticleStatusTagType,
  type CareArticle,
  type CareArticleStatus,
} from '@/constants/article'
import { formatDate } from '@/utils/dateFormat'
import type { UserInfo } from '@/types/api'

const userStore = useUserStore()
const profile = ref<UserInfo | null>(null)
const favorites = ref<Favorite[]>([])
const myArticles = ref<CareArticle[]>([])
const draftFilter = ref<CareArticleStatus | ''>('')
const form = reactive({ nickname: '', bio: '', avatar: '' })

onMounted(async () => {
  await userStore.load()
  profile.value = userStore.profile
  form.nickname = profile.value?.nickname || ''
  form.bio = profile.value?.bio || ''
  form.avatar = profile.value?.avatar || ''
  favorites.value = await listFavorites()
  await loadMyArticles()
})

async function loadMyArticles() {
  const res = await listMyArticles({ status: draftFilter.value })
  myArticles.value = res.list
}

async function save() {
  await userStore.save({ nickname: form.nickname, bio: form.bio, avatar: form.avatar })
  profile.value = userStore.profile
  ElMessage.success('资料已更新')
}
</script>

<style scoped>
.page { max-width: 1000px; margin: 0 auto; }
.avatar { margin-bottom: 12px; }
.profile-form { margin-top: 12px; }
.fav-card { margin-bottom: 16px; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.draft-card :deep(.el-radio-button__inner) { padding: 6px 10px; }
</style>
