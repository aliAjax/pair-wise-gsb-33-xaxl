export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

export interface PageData<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface UserInfo {
  id: number
  username: string
  email: string
  nickname: string
  avatar: string
  bio: string
  role: 'user' | 'admin'
  created_at: string
}

export interface CareReminder {
  id: number
  user_id: number
  plant_species_id: number
  task_title: string
  remind_date: string
  frequency: string
  status: 'pending' | 'done' | 'overdue'
  created_at: string
}

export interface UserGarden {
  id: number
  user_id: number
  plant_species_id: number
  nickname: string
  owned_since: string
  location: string
  care_reminder_id: number
  created_at: string
}

export interface DiseasePest {
  id: number
  plant_species_id: number
  name: string
  symptoms: string
  cause: string
  treatment: string
  recommended_medicine: string
  images: string
  keywords: string
  created_at: string
}

export interface Question {
  id: number
  user_id: number
  title: string
  content: string
  images: string
  status: 'open' | 'closed'
  created_at: string
}

export interface Answer {
  id: number
  question_id: number
  user_id: number
  content: string
  is_best: boolean
  like_count: number
  version: number
  created_at: string
  updated_at: string
}

export interface AdoptResult {
  best_answer: Answer
  draft: CareArticleLike
}

// Structural type for the care article fields shared with the Q&A module, so
// api.ts does not have to import constants (avoiding a constants <-> types
// cycle). Use constants/article.ts CareArticle elsewhere.
export interface CareArticleLike {
  id: number
  user_id: number
  title: string
  content: string
  cover: string
  topic_tag: string
  status: 'draft' | 'published' | 'invalidated'
  view_count: number
  source_question_id: number
  source_answer_id: number
  question_snapshot: string
  answer_snapshot: string
  answer_version: number
  created_at: string
  updated_at: string
}
