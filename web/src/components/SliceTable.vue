<script setup lang="ts">
import { columnsFrom } from '../format'
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
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.id"
          :class="{ on: props.activeId === row.id }"
          @click="emit('select', row.id)"
        >
          <td class="name">{{ row.label }}</td>
          <td v-for="(cell, i) in columnsFrom(row)" :key="i" class="num">{{ cell }}</td>
          <td class="num">{{ row.requests }}</td>
          <td v-if="props.showTurns" class="num">{{ row.user_turns }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
