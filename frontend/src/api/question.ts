import request from '@/utils/request'
import type { Question, Answer, AdoptResult, CareArticleLike } from '@/types/api'
import type { PageData } from '@/types/api'

export function listQuestions(params: { page?: number; page_size?: number }) {
  return request.get<never, PageData<Question>>('/questions', { params })
}

export function getQuestion(id: number | string) {
  return request.get<never, Question>(`/questions/${id}`)
}

export function createQuestion(payload: { title: string; content: string; images?: string }) {
  return request.post<never, Question>('/questions', payload)
}

export function listAnswers(questionId: number) {
  return request.get<never, Answer[]>(`/questions/${questionId}/answers`)
}

export function createAnswer(questionId: number, content: string) {
  return request.post<never, Answer>(`/questions/${questionId}/answers`, { content })
}

export function updateAnswer(answerId: number, content: string) {
  return request.put<never, Answer>(`/answers/${answerId}`, { content })
}

// Adopt an answer. Returns the best answer together with the care article
// draft generated atomically by the backend.
export function adoptAnswer(questionId: number, answerId: number) {
  return request.put<never, AdoptResult>(`/questions/${questionId}/adopt`, { answer_id: answerId })
}

// The sedimentation draft of the question owner's closed question.
export function getQuestionDraft(questionId: number) {
  return request.get<never, CareArticleLike>(`/questions/${questionId}/draft`)
}

// Regenerate an invalidated/missing draft from the current best answer.
export function retryQuestionDraft(questionId: number) {
  return request.put<never, CareArticleLike>(`/questions/${questionId}/draft/retry`)
}

export function likeAnswer(answerId: number) {
  return request.put<never, Answer>(`/answers/${answerId}/like`)
}
