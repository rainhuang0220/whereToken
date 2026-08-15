<script setup lang="ts">
import { columnsFrom } from '../format'
import type { DrillTables, SessionView, SliceView } from '../types'

defineProps<{
  pack: DrillTables
}>()

const heads = ['未命中', '缓存读', '缓存写', '输出', '合计', '命中率']

function sessionLabel(row: SessionView): string {
  const id = row.label || row.id
  const short = id.length > 22 ? id.slice(0, 10) + '…' + id.slice(-8) : id
  return short
}
</script>

<template>
  <section class="drill" aria-label="下钻">
    <div class="drill-grid">
      <section class="ledger">
        <h2>按模型</h2>
        <table>
          <thead>
            <tr>
              <th class="name">模型</th>
              <th v-for="h in heads" :key="h" class="num">{{ h }}</th>
              <th class="num">请求</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in pack.models" :key="row.id">
              <td class="name">{{ row.label }}</td>
              <td v-for="(cell, i) in columnsFrom(row as SliceView)" :key="i" class="num">{{ cell }}</td>
              <td class="num">{{ row.requests }}</td>
            </tr>
          </tbody>
        </table>
      </section>
      <section class="ledger">
        <h2>按工作区</h2>
        <table>
          <thead>
            <tr>
              <th class="name">工作区</th>
              <th v-for="h in heads" :key="'w' + h" class="num">{{ h }}</th>
              <th class="num">请求</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in pack.workspaces" :key="row.id">
              <td class="name">{{ row.label }}</td>
              <td v-for="(cell, i) in columnsFrom(row as SliceView)" :key="i" class="num">{{ cell }}</td>
              <td class="num">{{ row.requests }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>
    <section class="ledger sessions">
      <h2>按会话</h2>
      <table>
        <thead>
          <tr>
            <th class="name">会话</th>
            <th class="name">模型</th>
            <th class="name">工作区</th>
            <th class="num">日期</th>
            <th v-for="h in heads" :key="'s' + h" class="num">{{ h }}</th>
            <th class="num">请求</th>
            <th class="num">回合</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in pack.sessions" :key="row.id">
            <td class="name">{{ sessionLabel(row) }}</td>
            <td class="name">{{ row.model }}</td>
            <td class="name">{{ row.workspace }}</td>
            <td class="num">{{ row.last_date || '—' }}</td>
            <td v-for="(cell, i) in columnsFrom(row)" :key="i" class="num">{{ cell }}</td>
            <td class="num">{{ row.requests }}</td>
            <td class="num">{{ row.user_turns }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </section>
</template>
