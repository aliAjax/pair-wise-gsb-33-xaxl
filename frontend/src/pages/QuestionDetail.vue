<template>
  <div class="page" v-if="question">
    <el-page-header @back="$router.back()" content="问题详情" />
    <el-card class="question-card">
      <h2>
        {{ question.title }}
        <el-tag v-if="question.status === 'closed'" type="info" size="small">已关闭</el-tag>
      </h2>
      <p class="content">{{ question.content }}</p>
      <p class="meta">{{ formatDateTime(question.created_at) }}</p>
    </el-card>
    <el-alert
      v-if="question.status === 'closed'"
      type="success"
      :closable="false"
      title="问题已采纳最佳回答并沉淀为文章草稿，问答内容已固定"
      class="closed-alert"
    />
    <el-card class="answers-card">
      <template #header>回答（{{ answers.length }}）</template>
      <div v-for="a in answers" :key="a.id" class="answer">
        <div class="answer-head">
          <span>用户 #{{ a.user_id }}</span>
          <el-tag v-if="a.is_best" type="success" size="small">最佳回答</el-tag>
          <span v-if="a.version > 1" class="version">v{{ a.version }}</span>
        </div>
        <p class="answer-content">{{ a.content }}</p>
        <div class="answer-actions">
          <el-button size="small" @click="like(a.id)">👍 {{ a.like_count }}</el-button>
          <el-button v-if="isOwner && !a.is_best && question.status !== 'closed'" size="small" type="warning" @click="adopt(a.id)">采纳为最佳</el-button>
          <el-button v-if="isAnswerOwner(a)" size="small" @click="openEdit(a)">编辑</el-button>
        </div>
      </div>
      <el-empty v-if="!answers.length" description="暂无回答" />
      <div v-if="question.status !== 'closed'" class="reply">
        <el-input v-model="reply" type="textarea" :rows="3" placeholder="写下你的回答…" />
        <el-button type="primary" :loading="replying" @click="submitReply">提交回答</el-button>
      </div>
    </el-card>

    <el-dialog v-model="editVisible" title="编辑回答" width="560px">
      <el-input v-model="editContent" type="textarea" :rows="5" />
      <el-alert
        v-if="editingAnswer?.is_best"
        type="warning"
        :closable="false"
        title="该回答已被采纳，修改后由其生成的未发布文章草稿将失效"
        class="edit-alert"
      />
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editing" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getQuestion, listAnswers, createAnswer, updateAnswer, adoptAnswer, likeAnswer } from '@/api/question'
import { useAuth } from '@/hooks/useAuth'
import { formatDateTime } from '@/utils/dateFormat'
import type { Answer, Question } from '@/types/api'

const route = useRoute()
const router = useRouter()
const { isLoggedIn, user } = useAuth()
const question = ref<Question | null>(null)
const answers = ref<Answer[]>([])
const reply = ref('')
const replying = ref(false)
const editVisible = ref(false)
const editContent = ref('')
const editingAnswer = ref<Answer | null>(null)
const editing = ref(false)

const isOwner = computed(() => !!user.value && question.value?.user_id === user.value.id)
const isAnswerOwner = (a: Answer) => !!user.value && a.user_id === user.value.id

onMounted(async () => {
  const id = Number(route.params.id)
  question.value = await getQuestion(id)
  answers.value = await listAnswers(id)
})

async function refresh() {
  const id = question.value!.id
  question.value = await getQuestion(id)
  answers.value = await listAnswers(id)
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
  try {
    const res = await adoptAnswer(question.value!.id, answerId)
    await refresh()
    ElMessage.success(res.draft ? '已采纳该回答，文章草稿已生成，可在个人中心继续编辑发布' : '已采纳该回答')
  } catch {
    // 另一个窗口可能已抢先采纳：刷新为最新状态
    await refresh()
  }
}
function openEdit(a: Answer) {
  editingAnswer.value = a
  editContent.value = a.content
  editVisible.value = true
}
async function submitEdit() {
  if (!editingAnswer.value || !editContent.value.trim()) return
  editing.value = true
  try {
    await updateAnswer(editingAnswer.value.id, editContent.value)
    editVisible.value = false
    answers.value = await listAnswers(question.value!.id)
    ElMessage.success(editingAnswer.value.is_best ? '回答已更新，关联的未发布草稿已失效' : '回答已更新')
  } finally {
    editing.value = false
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
</script>

<style scoped>
.page { max-width: 900px; margin: 0 auto; }
.question-card { margin-bottom: 16px; }
.content { line-height: 1.7; }
.meta { color: #999; font-size: 12px; }
.closed-alert { margin-bottom: 16px; }
.answers-card .answer { border-bottom: 1px solid #f0f0f0; padding: 12px 0; }
.answer-head { display: flex; gap: 8px; align-items: center; color: #888; font-size: 13px; }
.version { color: #bbb; font-size: 12px; }
.answer-content { line-height: 1.6; }
.answer-actions { display: flex; gap: 8px; }
.reply { margin-top: 16px; display: flex; flex-direction: column; gap: 12px; }
.edit-alert { margin-top: 12px; }
</style>
