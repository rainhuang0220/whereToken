<script setup lang="ts">
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { SliceView } from '../types'

echarts.use([BarChart, GridComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{ rows: SliceView[] }>()
const el = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

function render() {
  if (!chart) return
  const labels = props.rows.map((r) => r.label)
  const values = props.rows.map((r) => r.total)
  const texts = props.rows.map((r) => r.total_m)
  chart.setOption({
    grid: { left: 108, right: 72, top: 8, bottom: 8 },
    tooltip: { show: false },
    xAxis: { type: 'value', show: false },
    yAxis: {
      type: 'category',
      data: labels,
      inverse: true,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: '#3a342c', fontFamily: 'Noto Sans SC, sans-serif', fontSize: 12 },
    },
    series: [
      {
        type: 'bar',
        data: values,
        barWidth: 8,
        itemStyle: { color: '#1c1814', borderRadius: 0 },
        label: {
          show: true,
          position: 'right',
          formatter: (p: { dataIndex: number }) => texts[p.dataIndex],
          color: '#1c1814',
          fontFamily: 'Courier Prime, ui-monospace, monospace',
          fontSize: 12,
        },
      },
    ],
  })
}

onMounted(() => {
  if (!el.value) return
  chart = echarts.init(el.value, undefined, { renderer: 'canvas' })
  render()
})

watch(
  () => props.rows,
  () => render(),
  { deep: true },
)

onBeforeUnmount(() => {
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div ref="el" class="share" role="img" aria-label="按工具合计条形图" />
</template>
