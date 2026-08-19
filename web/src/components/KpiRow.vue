<script setup lang="ts">
import { computed } from 'vue'
import { rankHint } from '../community'
import { costHonestyNote, costKPI, formatCount, hitBand, optionalDay, rankCaption, tokenCell } from '../format'
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
  props.rankPeriod === 'all' ? props.community?.all : props.community?.today,
)
const hint = computed(() => rankHint(props.community, standing.value))
const honesty = computed(() => costHonestyNote(props.all))
const canJoin = computed(() => standing.value?.status === 'opted_out')
</script>

<template>
  <section class="readout" aria-label="合计">
    <div class="read-lead">
      <span class="read-k">合计</span>
      <strong>{{ tokenCell(all.total_m, all.quality, all) }}</strong>
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
      <strong>{{ optionalDay(todayM, all.quality) }}</strong>
    </div>
    <div
      class="read-cell read-col5"
      title="Estimated API-equivalent cost based on public model pricing. This is not your actual subscription bill."
    >
      <span class="read-k">估价</span>
      <strong>{{ costKPI(all) }}</strong>
      <span class="read-k rank-k">排名</span>
      <strong class="rank-v">{{ rankCaption(standing) }}</strong>
      <div class="rank-toggle" role="group" aria-label="社区排名范围">
        <button
          type="button"
          class="rank-btn"
          :class="{ on: rankPeriod !== 'all' }"
          :aria-pressed="rankPeriod !== 'all'"
          @click="emit('update:rankPeriod', 'today')"
        >
          今日
        </button>
        <button
          type="button"
          class="rank-btn"
          :class="{ on: rankPeriod === 'all' }"
          :aria-pressed="rankPeriod === 'all'"
          title="已同步的每日合计，不是窑墙「全部」"
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
      <button
        v-else-if="canJoin"
        type="button"
        class="rank-opt"
        @click="emit('toggle-community', true)"
      >
        参加社区
      </button>
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
      <strong>{{ optionalDay(peakM, all.quality) }}</strong>
    </div>
    <p v-if="compareText" class="read-compare">{{ compareText }}</p>
    <p v-if="honesty" class="read-compare">{{ honesty }}</p>
    <p v-if="hint" class="read-compare">{{ hint }}</p>
  </section>
</template>
