<template>
  <div class="page" v-if="question">
    <el-page-header @back="$router.back()" content="问题详情" />
    <el-card class="question-card">
      <div class="question-head">
        <h2>{{ question.title }}</h2>
        <el-tag :type="question.status === 'closed' ? 'success' : 'warning'" size="small">
          {{ question.status === 'closed' ? '已采纳最佳回答' : '待采纳' }}
        </el-tag>
      </div>
      <p class="content">{{ question.content }}</p>
      <p class="meta">{{ formatDateTime(question.created_at) }}</p>
    </el-card>

    <el-card v-if="isOwner && question.status === 'closed'" class="draft-card">
      <template #header>
        <div class="draft-head">
          <span>养护文章沉淀</span>
          <el-tag v-if="draft" :type="draftTagType" size="small">{{ draftStatusText }}</el-tag>
        </div>
      </template>
      <p v-if="draftLoading" class="draft-hint">草稿状态加载中…</p>
      <template v-else-if="draft">
        <p class="draft-title">{{ draft.title }}</p>
        <p class="draft-hint">
          <template v-if="draft.status === 'invalidated'">
            最佳回答已被修改，未发布的草稿已失效；已发布的养护文章会保留采纳时的原版内容。
          </template>
          <template v-else-if="draft.status === 'published'">
            养护文章已发布，页面展示的是采纳当时固定下来的问题与回答。
          </template>
          <template v-else>草稿根据采纳时的问题与回答自动生成，可前往完善后发布。</template>
        </p>
        <div class="draft-actions">
          <el-button v-if="draft.status === 'invalidated'" type="warning" :loading="draftRetrying" @click="retryDraft">
            重新生成草稿
          </el-button>
          <el-button v-if="draft.status === 'draft'" type="primary" @click="router.push(`/articles/${draft.id}`)">
            查看并完善草稿
          </el-button>
          <el-button v-if="draft.status === 'published'" type="success" @click="router.push(`/articles/${draft.id}`)">
            查看养护文章
          </el-button>
        </div>
      </template>
      <template v-else>
        <p class="draft-hint">草稿生成失败或尚未生成，可重新生成（不会改动已确定的最佳回答）。</p>
        <div class="draft-actions">
          <el-button type="warning" :loading="draftRetrying" @click="retryDraft">重新生成草稿</el-button>
        </div>
      </template>
    </el-card>

    <el-card class="answers-card">
      <template #header>回答（{{ answers.length }}）</template>
      <el-alert
        v-if="question.status === 'closed'"
        type="info"
        :closable="false"
        show-icon
        title="该问题已采纳最佳回答，采纳结果已固定；修改最佳回答只会使未发布的草稿失效，不会影响已发布文章。"
        class="closed-tip"
      />
      <div v-for="a in answers" :key="a.id" class="answer">
        <div class="answer-head">
          <span>用户 #{{ a.user_id }}</span>
          <el-tag v-if="a.is_best" type="success" size="small">最佳回答</el-tag>
          <el-tag v-if="isAnswerOwner(a) && a.is_best" type="info" size="small">已固定入草稿 v{{ a.version }}</el-tag>
        </div>
        <p class="answer-content">{{ a.content }}</p>
        <div class="answer-actions">
          <el-button size="small" @click="like(a.id)">👍 {{ a.like_count }}</el-button>
          <el-button v-if="isAnswerOwner(a)" size="small" @click="openEdit(a)">修改</el-button>
          <el-button
            v-if="isOwner && question.status === 'open' && !a.is_best"
            size="small"
            type="warning"
            :loading="adoptingId === a.id"
            @click="adopt(a.id)"
          >采纳为最佳</el-button>
        </div>
      </div>
      <el-empty v-if="!answers.length" description="暂无回答" />
      <div v-if="question.status === 'open'" class="reply">
        <el-input v-model="reply" type="textarea" :rows="3" placeholder="写下你的回答…" />
        <el-button type="primary" :loading="replying" @click="submitReply">提交回答</el-button>
      </div>
    </el-card>

    <el-dialog v-model="editVisible" title="修改回答" width="600px">
      <el-alert
        v-if="editingBest"
        type="warning"
        :closable="false"
        show-icon
        title="该回答已被采纳为最佳回答：保存修改后，未发布的养护文章草稿将失效（需重新生成）；已发布文章保留原版。"
        class="edit-warn"
      />
      <el-input v-model="editContent" type="textarea" :rows="5" />
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editing" @click="saveEdit">保存修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  getQuestion,
  listAnswers,
  createAnswer,
  updateAnswer,
  adoptAnswer,
  likeAnswer,
  getQuestionDraft,
  retryQuestionDraft,
} from '@/api/question'
import { useAuth } from '@/hooks/useAuth'
import { formatDateTime } from '@/utils/dateFormat'
import { CareArticleStatusMap, CareArticleStatusTagType } from '@/constants/article'
import type { Answer, CareArticleLike, Question } from '@/types/api'

const route = useRoute()
const router = useRouter()
const { isLoggedIn, user } = useAuth()
const question = ref<Question | null>(null)
const answers = ref<Answer[]>([])
const reply = ref('')
const replying = ref(false)
const adoptingId = ref<number | null>(null)
const draft = ref<CareArticleLike | null>(null)
const draftLoading = ref(false)
const draftRetrying = ref(false)

const isOwner = computed(() => !!user.value && question.value?.user_id === user.value.id)
const draftStatusText = computed(() => (draft.value ? CareArticleStatusMap[draft.value.status] : ''))
const draftTagType = computed(() => (draft.value ? CareArticleStatusTagType[draft.value.status] : 'info'))

function isAnswerOwner(a: Answer) {
  return !!user.value && a.user_id === user.value.id
}

onMounted(loadPage)

async function loadPage() {
  const id = Number(route.params.id)
  question.value = await getQuestion(id)
  answers.value = await listAnswers(id)
  if (isOwner.value && question.value.status === 'closed') {
    await loadDraft()
  }
}

async function loadDraft() {
  draftLoading.value = true
  try {
    draft.value = await getQuestionDraft(Number(route.params.id))
  } catch {
    // 404: adoption draft missing and can be regenerated.
    draft.value = null
  } finally {
    draftLoading.value = false
  }
}

async function submitReply() {
  if (!isLoggedIn.value) {
    ElMessage.warning('请先登录')
    router.push('/login')
    return
  }
  if (!reply.value.trim()) return
  replying.value = true
  try {
    await createAnswer(question.value!.id, reply.value)
    answers.value = await listAnswers(question.value!.id)
    reply.value = ''
  } finally {
    replying.value = false
  }
}

async function adopt(answerId: number) {
  adoptingId.value = answerId
  try {
    const result = await adoptAnswer(question.value!.id, answerId)
    answers.value = await listAnswers(question.value!.id)
    question.value = await getQuestion(question.value!.id)
    draft.value = result.draft
    ElMessage.success('已采纳该回答，养护文章草稿已生成')
  } catch {
    // 两个窗口同时采纳时，只让先到的一版成为最佳回答；后到者刷新到最新状态。
    question.value = await getQuestion(question.value!.id)
    answers.value = await listAnswers(question.value!.id)
    await loadDraft()
  } finally {
    adoptingId.value = null
  }
}

async function like(answerId: number) {
  if (!isLoggedIn.value) {
    ElMessage.warning('请先登录')
    router.push('/login')
    return
  }
  await likeAnswer(answerId)
  answers.value = await listAnswers(question.value!.id)
}

const editVisible = ref(false)
const editContent = ref('')
const editing = ref(false)
const editingId = ref<number | null>(null)
const editingBest = ref(false)

function openEdit(a: Answer) {
  editingId.value = a.id
  editingBest.value = a.is_best
  editContent.value = a.content
  editVisible.value = true
}

async function saveEdit() {
  if (!editContent.value.trim() || editingId.value === null) return
  editing.value = true
  try {
    await updateAnswer(editingId.value, editContent.value)
    answers.value = await listAnswers(question.value!.id)
    editVisible.value = false
    if (editingBest.value) {
      ElMessage.warning('回答已修改，未发布草稿已失效，可重新生成')
      await loadDraft()
    } else {
      ElMessage.success('回答已修改')
    }
  } finally {
    editing.value = false
  }
}

async function retryDraft() {
  draftRetrying.value = true
  try {
    draft.value = await retryQuestionDraft(Number(route.params.id))
    ElMessage.success('草稿已重新生成')
  } finally {
    draftRetrying.value = false
  }
}
</script>

<style scoped>
.page { max-width: 900px; margin: 0 auto; }
.question-card { margin-bottom: 16px; }
.question-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.content { line-height: 1.7; }
.meta { color: #999; font-size: 12px; }
.draft-card { margin-bottom: 16px; }
.draft-head { display: flex; justify-content: space-between; align-items: center; }
.draft-title { font-weight: 600; margin: 0 0 8px; }
.draft-hint { color: #888; font-size: 13px; line-height: 1.6; margin: 0 0 12px; }
.draft-actions { display: flex; gap: 8px; }
.answers-card .answer { border-bottom: 1px solid #f0f0f0; padding: 12px 0; }
.answer-head { display: flex; gap: 8px; align-items: center; color: #888; font-size: 13px; }
.answer-content { line-height: 1.6; }
.answer-actions { display: flex; gap: 8px; }
.closed-tip { margin-bottom: 12px; }
.reply { margin-top: 16px; display: flex; flex-direction: column; gap: 12px; }
.edit-warn { margin-bottom: 12px; }
</style>
