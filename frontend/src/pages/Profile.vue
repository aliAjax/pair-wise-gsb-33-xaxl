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
        <el-card>
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
      </el-col>
    </el-row>
    <el-card class="articles-card">
      <template #header>我的文章（含问答沉淀草稿）</template>
      <el-table :data="articles" empty-text="暂无文章">
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column label="主题" width="100">
          <template #default="{ row }">{{ CareTopicTagMap[row.topic_tag as CareTopicTag] }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" size="small">{{ CareArticleStatusMap[row.status as CareArticleStatus] || row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="来源" width="90">
          <template #default="{ row }">{{ row.question_id ? `问答 #${row.question_id}` : '手动撰写' }}</template>
        </el-table-column>
        <el-table-column label="更新时间" width="110">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220">
          <template #default="{ row }">
            <el-button v-if="row.status === 'published'" size="small" @click="$router.push(`/articles/${row.id}`)">查看</el-button>
            <el-button v-if="row.status === 'draft'" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.status === 'draft'" size="small" type="success" @click="publish(row)">发布</el-button>
            <el-tooltip v-if="row.status === 'invalid'" content="关联回答已修改，草稿快照过期，不可再编辑发布" placement="top">
              <el-tag size="small" type="danger">快照已过期</el-tag>
            </el-tooltip>
            <el-button v-if="row.status !== 'published'" size="small" type="danger" plain @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="articleTotal > 0" layout="prev, pager, next" :total="articleTotal" :page-size="articlePageSize" :current-page="articlePage" @current-change="onArticlePage" class="pager" />
    </el-card>

    <el-dialog v-model="editVisible" title="编辑文章草稿" width="640px">
      <el-form label-width="80px">
        <el-form-item label="标题"><el-input v-model="editForm.title" /></el-form-item>
        <el-form-item label="主题">
          <el-select v-model="editForm.topic_tag">
            <el-option v-for="t in TOPIC_TAGS" :key="t" :label="CareTopicTagMap[t]" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容"><el-input v-model="editForm.content" type="textarea" :rows="10" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button :loading="saving" @click="saveEdit(false)">保存草稿</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit(true)">保存并发布</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import ImageUploader from '@/components/common/ImageUploader.vue'
import { useUserStore } from '@/stores/userStore'
import { listFavorites } from '@/api/favorite'
import { listMyArticles, updateArticle, deleteArticle } from '@/api/article'
import { FavoriteTargetTypeMap, type Favorite, type FavoriteTargetType } from '@/constants/favorite'
import { CareTopicTagMap, CareArticleStatusMap, TOPIC_TAGS, type CareArticle, type CareTopicTag, type CareArticleStatus } from '@/constants/article'
import { formatDate } from '@/utils/dateFormat'
import type { UserInfo } from '@/types/api'

const userStore = useUserStore()
const profile = ref<UserInfo | null>(null)
const favorites = ref<Favorite[]>([])
const articles = ref<CareArticle[]>([])
const articleTotal = ref(0)
const articlePage = ref(1)
const articlePageSize = 10
const form = reactive({ nickname: '', bio: '', avatar: '' })
const editVisible = ref(false)
const saving = ref(false)
const editingId = ref(0)
const editForm = reactive({ title: '', content: '', topic_tag: 'qa' as CareTopicTag })

onMounted(async () => {
  await userStore.load()
  profile.value = userStore.profile
  form.nickname = profile.value?.nickname || ''
  form.bio = profile.value?.bio || ''
  form.avatar = profile.value?.avatar || ''
  favorites.value = await listFavorites()
  await loadArticles()
})

async function loadArticles() {
  const res = await listMyArticles({ page: articlePage.value, page_size: articlePageSize })
  articles.value = res.list
  articleTotal.value = res.total
}
function onArticlePage(p: number) {
  articlePage.value = p
  loadArticles()
}
function statusTagType(status: string) {
  if (status === 'published') return 'success'
  if (status === 'draft') return 'warning'
  return 'danger'
}
function openEdit(a: CareArticle) {
  editingId.value = a.id
  editForm.title = a.title
  editForm.content = a.content
  editForm.topic_tag = a.topic_tag
  editVisible.value = true
}
async function saveEdit(publishAfter: boolean) {
  saving.value = true
  try {
    await updateArticle(editingId.value, {
      title: editForm.title,
      content: editForm.content,
      topic_tag: editForm.topic_tag,
      ...(publishAfter ? { status: 'published' } : {}),
    })
    editVisible.value = false
    await loadArticles()
    ElMessage.success(publishAfter ? '文章发布成功' : '文章已保存')
  } finally {
    saving.value = false
  }
}
async function publish(a: CareArticle) {
  await updateArticle(a.id, { status: 'published' })
  await loadArticles()
  ElMessage.success('文章发布成功')
}
async function remove(a: CareArticle) {
  await ElMessageBox.confirm(`确定删除文章「${a.title}」吗？`, '删除确认', { type: 'warning' })
  await deleteArticle(a.id)
  await loadArticles()
  ElMessage.success('已删除')
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
.articles-card { margin-top: 16px; }
.pager { margin-top: 16px; justify-content: center; }
</style>
