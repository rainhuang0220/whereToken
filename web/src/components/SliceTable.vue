<script setup lang="ts">
import { columnsFrom, costCaption, derivationCaption, formatCount, hitBand, qualityCaption } from '../format'
import { isRowActivateKey, rowIsSelectable } from '../sliceTable'
import type { SliceView } from '../types'

const props = defineProps<{
  title: string
  rows: SliceView[]
  showTurns?: boolean
  activeId?: string
}>()

const emit = defineEmits<{
  select: [id: string]
}>()

const heads = ['未命中', '缓存读', '缓存写', '输出', '合计', '命中率']

function activate(id: string, quality: string) {
  if (!rowIsSelectable(quality)) return
  emit('select', id)
}

function onRowKey(e: KeyboardEvent, id: string, quality: string) {
  if (!isRowActivateKey(e.key)) return
  e.preventDefault()
  activate(id, quality)
}
</script>

<template>
  <section class="ledger">
    <h2>{{ title }}</h2>
    <table>
      <thead>
        <tr>
          <th class="name">名称</th>
          <th v-for="h in heads" :key="h" class="num">{{ h }}</th>
          <th class="num">请求</th>
          <th v-if="props.showTurns" class="num">回合</th>
          <th class="num">估价</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.id"
          :class="{ on: props.activeId === row.id, absent: row.quality === 'absent' }"
          :tabindex="row.quality === 'absent' ? -1 : 0"
          :role="row.quality === 'absent' ? undefined : 'button'"
          @click="activate(row.id, row.quality)"
          @keydown="onRowKey($event, row.id, row.quality)"
        >
          <td class="name">
            {{ row.label }}
            <span v-if="qualityCaption(row.quality) || derivationCaption(row.derivation || '')" class="qual">
              {{ [qualityCaption(row.quality), derivationCaption(row.derivation || ''), row.error].filter(Boolean).join(' · ') }}
            </span>
          </td>
          <td
            v-for="(cell, i) in columnsFrom(row)"
            :key="i"
            class="num"
            :data-hit="i === 5 ? hitBand(row.hit_rate_text) : undefined"
          >
            {{ cell }}
          </td>
          <td class="num">{{ formatCount(row.requests) }}</td>
          <td v-if="props.showTurns" class="num">{{ formatCount(row.user_turns) }}</td>
          <td class="num">{{ costCaption(row) || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
