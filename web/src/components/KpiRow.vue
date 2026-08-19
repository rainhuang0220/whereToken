<script setup lang="ts">
import { computed } from 'vue'
import { costKPI, formatCount, hitBand, rankCaption } from '../format'
import type { CommunityView, RankPeriod, SliceView } from '../types'

const props = defineProps<{
  all: SliceView
  todayM?: string
  peakM?: string
  maxStreak?: number
  currentStreak?: number
  compareText?: string
  community?: CommunityView
  rankPeriod?: RankPeriod
}>()

const emit = defineEmits<{
  'update:rankPeriod': [period: RankPeriod]
  'toggle-community': [enabled: boolean]
}>()

const standing = computed(() =>
  props.rankPeriod === 'today' ? props.community?.today : props.community?.all,
)
</script>

<template>
  <section class="readout" aria-label="合计">
    <div class="read-lead">
      <span class="read-k">合计</span>
      <strong>{{ all.total_m }}</strong>
    </div>
    <div class="read-cell" :data-hit="hitBand(all.hit_rate_text)">
      <span class="read-k">命中率</span>
      <strong>{{ all.hit_rate_text }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">最长连烧</span>
      <strong>{{ formatCount(maxStreak || 0) }} 天</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">当日用量</span>
      <strong>{{ todayM || '0.00 M' }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">估价</span>
      <strong>{{ costKPI(all) }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">当前连烧</span>
      <strong>{{ formatCount(currentStreak || 0) }} 天</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">请求</span>
      <strong>{{ formatCount(all.requests) }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">用户回合</span>
      <strong>{{ formatCount(all.user_turns) }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">单日最高</span>
      <strong>{{ peakM || '0.00 M' }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">排名</span>
      <strong class="rank-v">{{ rankCaption(standing) }}</strong>
      <div class="rank-toggle">
        <button
          type="button"
          class="rank-btn"
          :class="{ on: rankPeriod !== 'all' }"
          @click="emit('update:rankPeriod', 'today')"
        >
          今日
        </button>
        <button
          type="button"
          class="rank-btn"
          :class="{ on: rankPeriod === 'all' }"
          @click="emit('update:rankPeriod', 'all')"
        >
          累计
        </button>
      </div>
      <button
        v-if="community?.enabled"
        type="button"
        class="rank-opt"
        @click="emit('toggle-community', false)"
      >
        退出社区
      </button>
    </div>
    <p v-if="compareText" class="read-compare">{{ compareText }}</p>
    <p v-if="all.cost_status === 'partial' && all.cost_usd" class="read-compare">
      估价 {{ all.cost_usd }} · 部分无标价 · 不是账单
    </p>
    <p v-else-if="all.cost_status === 'unavailable' && all.total > 0" class="read-compare">
      估价不可用 · 不会写成 $0
    </p>
  </section>
</template>
