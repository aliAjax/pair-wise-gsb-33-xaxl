export type CareTopicTag = 'fertilizing' | 'pruning' | 'repotting' | 'pest_control' | 'propagation'

export const CareTopicTagMap: Record<CareTopicTag, string> = {
  fertilizing: '施肥',
  pruning: '修剪',
  repotting: '换盆',
  pest_control: '病虫害',
  propagation: '繁殖',
}

export const TOPIC_TAGS = Object.keys(CareTopicTagMap) as CareTopicTag[]

export type CareArticleStatus = 'draft' | 'published' | 'invalidated'

export const CareArticleStatusMap: Record<CareArticleStatus, string> = {
  draft: '草稿',
  published: '已发布',
  invalidated: '已失效',
}

export const CareArticleStatusTagType: Record<CareArticleStatus, 'info' | 'success' | 'danger'> = {
  draft: 'info',
  published: 'success',
  invalidated: 'danger',
}

export interface CareArticle {
  id: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag | ''
  status: CareArticleStatus
  view_count: number
  source_question_id: number
  source_answer_id: number
  question_snapshot: string
  answer_snapshot: string
  answer_version: number
  created_at: string
  updated_at: string
}
