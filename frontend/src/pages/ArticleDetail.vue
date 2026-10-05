<template>
  <div class="page" v-if="article">
    <el-page-header @back="$router.back()" content="文章详情" />
    <el-card class="article">
      <div class="title-row">
        <h1>{{ article.title }}</h1>
        <el-tag :type="statusTagType" size="small">{{ statusText }}</el-tag>
      </div>
      <div class="meta">
        <el-tag v-if="article.topic_tag" type="success">{{ CareTopicTagMap[article.topic_tag as CareTopicTag] }}</el-tag>
        <span v-if="article.status === 'published'">
          发布于 {{ formatDateTime(article.created_at) }} · 阅读 {{ article.view_count }}
        </span>
        <span v-else>生成于 {{ formatDateTime(article.created_at) }}</span>
        <FavoriteButton v-if="article.status === 'published'" target-type="article" :target-id="article.id" />
      </div>

      <el-alert
        v-if="article.source_question_id && article.status === 'published'"
        type="success"
        :closable="false"
        show-icon
        title="本文由问答采纳沉淀而来，内容为采纳当时固定的问题与最佳回答；之后回答再被修改也不会改变本文。"
        class="source-tip"
      />
      <el-alert
        v-else-if="article.source_question_id && article.status === 'invalidated'"
        type="error"
        :closable="false"
        show-icon
        title="最佳回答已被修改，此草稿已失效。请回到问题页重新生成草稿；已发布的养护文章始终保留原版。"
        class="source-tip"
      />
      <el-alert
        v-else-if="article.source_question_id && article.status === 'draft'"
        type="info"
        :closable="false"
        show-icon
        title="此草稿由问答采纳自动生成，已固定采纳时的问题与回答。可继续完善并选择主题后发布。"
        class="source-tip"
      />

      <el-image v-if="article.cover" :src="article.cover" fit="cover" class="cover" />
      <div class="content" v-html="renderedContent"></div>

      <div v-if="isOwner && article.status === 'draft'" class="owner-actions">
        <el-button @click="openEdit">完善草稿</el-button>
        <el-button type="primary" @click="openPublish">发布为养护文章</el-button>
      </div>
      <div v-if="isOwner && article.status === 'invalidated'" class="owner-actions">
        <el-button type="warning" @click="goQuestion">回问题页重新生成</el-button>
      </div>
    </el-card>

    <el-dialog v-model="editVisible" title="完善草稿" width="720px">
      <el-form label-width="72px">
        <el-form-item label="标题"><el-input v-model="editForm.title" maxlength="255" show-word-limit /></el-form-item>
        <el-form-item label="封面"><el-input v-model="editForm.cover" placeholder="封面图片地址（可选）" /></el-form-item>
        <el-form-item label="正文"><el-input v-model="editForm.content" type="textarea" :rows="10" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveDraft">保存草稿</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="publishVisible" title="发布养护文章" width="480px">
      <p class="publish-hint">发布后文章将以当前草稿内容固定展示，之后最佳回答被修改也不会影响文章原文。</p>
      <el-select v-model="publishTopic" placeholder="选择养护主题" class="publish-select">
        <el-option v-for="t in TOPIC_TAGS" :key="t" :label="CareTopicTagMap[t]" :value="t" />
      </el-select>
      <template #footer>
        <el-button @click="publishVisible = false">取消</el-button>
        <el-button type="primary" :loading="publishing" :disabled="!publishTopic" @click="publish">确认发布</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getArticle, updateArticle, publishArticleDraft } from '@/api/article'
import FavoriteButton from '@/components/common/FavoriteButton.vue'
import {
  CareTopicTagMap,
  TOPIC_TAGS,
  CareArticleStatusMap,
  CareArticleStatusTagType,
  type CareArticle,
  type CareTopicTag,
} from '@/constants/article'
import { useAuth } from '@/hooks/useAuth'
import { formatDateTime } from '@/utils/dateFormat'

const route = useRoute()
const router = useRouter()
const { user } = useAuth()
const article = ref<CareArticle | null>(null)

const isOwner = computed(() => !!user.value && article.value?.user_id === user.value.id)
const statusText = computed(() => (article.value ? CareArticleStatusMap[article.value.status] : ''))
const statusTagType = computed(() => (article.value ? CareArticleStatusTagType[article.value.status] : 'info'))

const renderedContent = computed(() => {
  const raw = article.value?.content || ''
  return raw.split('\n').map((p) => `<p>${p}</p>`).join('')
})

onMounted(load)

async function load() {
  article.value = await getArticle(route.params.id as string)
}

const editVisible = ref(false)
const saving = ref(false)
const editForm = reactive({ title: '', content: '', cover: '' })

function openEdit() {
  editForm.title = article.value!.title
  editForm.content = article.value!.content
  editForm.cover = article.value!.cover
  editVisible.value = true
}

async function saveDraft() {
  if (!editForm.title.trim() || !editForm.content.trim()) {
    ElMessage.warning('标题和正文不能为空')
    return
  }
  saving.value = true
  try {
    await updateArticle(article.value!.id, {
      title: editForm.title,
      content: editForm.content,
      cover: editForm.cover,
      topic_tag: article.value!.topic_tag,
    })
    ElMessage.success('草稿已保存')
    editVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

const publishVisible = ref(false)
const publishing = ref(false)
const publishTopic = ref<CareTopicTag | ''>('')

function openPublish() {
  publishTopic.value = article.value!.topic_tag || ''
  publishVisible.value = true
}

async function publish() {
  if (!publishTopic.value) return
  publishing.value = true
  try {
    await publishArticleDraft(article.value!.id, publishTopic.value)
    ElMessage.success('养护文章已发布')
    publishVisible.value = false
    await load()
  } finally {
    publishing.value = false
  }
}

function goQuestion() {
  router.push(`/questions/${article.value!.source_question_id}`)
}
</script>

<style scoped>
.page { max-width: 900px; margin: 0 auto; }
.article { padding: 16px; }
.title-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.meta { display: flex; align-items: center; gap: 12px; color: #999; margin: 12px 0; }
.source-tip { margin: 12px 0; }
.cover { width: 100%; max-height: 400px; border-radius: 8px; margin: 12px 0; }
.content { line-height: 1.9; color: #333; }
.owner-actions { margin-top: 24px; display: flex; gap: 12px; justify-content: flex-end; }
.publish-hint { color: #888; font-size: 13px; line-height: 1.6; }
.publish-select { width: 100%; margin-top: 12px; }
</style>
