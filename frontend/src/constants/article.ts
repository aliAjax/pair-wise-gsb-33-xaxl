export type CareTopicTag = 'fertilizing' | 'pruning' | 'repotting' | 'pest_control' | 'propagation' | 'qa'

export const CareTopicTagMap: Record<CareTopicTag, string> = {
  fertilizing: '施肥',
  pruning: '修剪',
  repotting: '换盆',
  pest_control: '病虫害',
  propagation: '繁殖',
  qa: '问答精选',
}

export const TOPIC_TAGS = Object.keys(CareTopicTagMap) as CareTopicTag[]

export type CareArticleStatus = 'draft' | 'published' | 'invalid'

export const CareArticleStatusMap: Record<CareArticleStatus, string> = {
  draft: '草稿',
  published: '已发布',
  invalid: '已失效',
}

export interface CareArticle {
  id: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: CareTopicTag
  status: string
  view_count: number
  question_id: number
  answer_id: number
  answer_version: number
  created_at: string
  updated_at: string
}
