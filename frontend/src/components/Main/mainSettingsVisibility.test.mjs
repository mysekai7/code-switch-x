import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const readMainComponent = () => readFile(new URL('./Index.vue', import.meta.url), 'utf8')
const readGlobalStyle = () => readFile(new URL('../../style.css', import.meta.url), 'utf8')
const readUsageHeatmapData = () => readFile(new URL('../../data/usageHeatmap.ts', import.meta.url), 'utf8')

const cssRule = (style, selector) => {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = style.match(new RegExp(`${escapedSelector}\\s*{([\\s\\S]*?)\\n}`))
  assert.ok(match, `missing CSS rule: ${selector}`)
  return match[1]
}

test('main page hides optional hero modules until app settings are loaded', async () => {
  const component = await readMainComponent()

  assert.match(component, /const appSettingsLoaded = ref\(false\)/)
  assert.match(component, /const showHeatmap = ref\(false\)/)
  assert.match(component, /const showHomeTitle = ref\(false\)/)
  assert.match(
    component,
    /const shouldShowHeatmap = computed\(\(\) => appSettingsLoaded\.value && showHeatmap\.value\)/,
  )
  assert.match(
    component,
    /const shouldShowHomeTitle = computed\(\(\) => appSettingsLoaded\.value && showHomeTitle\.value\)/,
  )
  assert.match(component, /<h1 v-if="shouldShowHomeTitle"/)
  assert.match(component, /v-if="shouldShowHeatmap"/)
  assert.match(component, /finally\s*{[\s\S]*appSettingsLoaded\.value = true/)
  assert.match(component, /await loadAppSettings\(\)[\s\S]*if \(showHeatmap\.value\) {[\s\S]*void loadUsageHeatmap\(\)/)
  assert.doesNotMatch(component, /const showHeatmap = ref\(true\)/)
  assert.doesNotMatch(component, /const showHomeTitle = ref\(true\)/)
})

test('main page restores selected provider tab after route remounts', async () => {
  const component = await readMainComponent()

  assert.match(component, /const MAIN_PROVIDER_TAB_STORAGE_KEY = 'code-switch-x-main-provider-tab'/)
  assert.match(component, /const readStoredProviderTab = \(\): ProviderTab =>/)
  assert.match(component, /localStorage\.getItem\(MAIN_PROVIDER_TAB_STORAGE_KEY\)/)
  assert.match(component, /providerTabIds\.includes\(stored as ProviderTab\)/)
  assert.match(component, /const selectedIndex = ref\(providerTabIds\.indexOf\(readStoredProviderTab\(\)\)\)/)
  assert.match(component, /const persistSelectedProviderTab = \(tab: ProviderTab\) =>/)
  assert.match(component, /localStorage\.setItem\(MAIN_PROVIDER_TAB_STORAGE_KEY, tab\)/)
  assert.match(component, /persistSelectedProviderTab\(nextTab\)/)
  assert.doesNotMatch(component, /const selectedIndex = ref\(0\)/)
})

test('main usage heatmap uses a GitHub-like contribution graph skin', async () => {
  const [component, style] = await Promise.all([readMainComponent(), readGlobalStyle()])
  const wallRule = cssRule(style, '.contrib-wall')
  const calendarRule = cssRule(style, '.contrib-calendar')
  const gridShellRule = cssRule(style, '.contrib-grid-shell')
  const gridRule = cssRule(style, '.contrib-grid')
  const cellRule = cssRule(style, '.contrib-cell')
  const hoverRule = cssRule(style, '.contrib-cell:hover')
  const footerRule = cssRule(style, '.contrib-footer')
  const intensityRule = cssRule(style, '.contrib-intensity')

  assert.match(component, /class="contrib-calendar"/)
  assert.match(component, /class="contrib-grid-shell"/)
  assert.match(component, /class="contrib-footer"/)
  assert.match(component, /class="contrib-help"/)
  assert.match(component, /class="contrib-intensity"/)

  assert.match(wallRule, /border-radius:\s*6px/)
  assert.match(wallRule, /background:\s*var\(--contrib-surface/)
  assert.match(calendarRule, /overflow-x:\s*auto/)
  assert.match(gridShellRule, /display:\s*block/)
  assert.match(gridShellRule, /width:\s*max-content/)
  assert.match(gridShellRule, /margin:\s*0 auto/)
  assert.match(gridRule, /justify-content:\s*flex-start/)
  assert.match(cellRule, /width:\s*10px/)
  assert.match(cellRule, /height:\s*10px/)
  assert.match(cellRule, /border-radius:\s*2px/)
  assert.doesNotMatch(hoverRule, /transform:\s*scale/)
  assert.match(footerRule, /justify-content:\s*space-between/)
  assert.match(intensityRule, /justify-content:\s*flex-end/)
})

test('main usage heatmap shows compact grouped axes for hourly data', async () => {
  const [component, style, heatmapData] = await Promise.all([
    readMainComponent(),
    readGlobalStyle(),
    readUsageHeatmapData(),
  ])
  const axisXRule = cssRule(style, '.contrib-axis-x')
  const axisYRule = cssRule(style, '.contrib-axis-y')
  const axisXGroupsRule = cssRule(style, '.contrib-axis-x-groups')
  const axisXLabelRule = cssRule(style, '.contrib-axis-x-label')
  const axisYLabelRule = cssRule(style, '.contrib-axis-y-label')
  const bucketAxisRule = cssRule(style, '.contrib-axis-buckets')
  const bucketLabelRule = cssRule(style, '.contrib-axis-bucket-label')
  const compactGridRule = cssRule(style, '.contrib-grid-compact')
  const dayBoundaryRule = cssRule(style, '.contrib-day-boundary')

  assert.match(component, /HEATMAP_ROWS/)
  assert.match(component, /BUCKETS_PER_DAY/)
  assert.match(component, /const HEATMAP_DATE_LABEL_INTERVAL = 3/)
  assert.match(heatmapData, /export const HEATMAP_ROWS = 6/)
  assert.match(heatmapData, /export const BUCKETS_PER_DAY = 4/)
  assert.match(component, /const heatmapDayGroups = computed/)
  assert.match(component, /const heatmapBucketTicks = computed/)
  assert.match(component, /const heatmapHourOffsetTicks = computed/)
  assert.doesNotMatch(component, /HEATMAP_HOURS/)
  assert.match(component, /class="contrib-axis-x"/)
  assert.match(component, /class="contrib-axis-x-groups"/)
  assert.match(component, /class="contrib-axis-buckets"/)
  assert.match(component, /class="contrib-axis-y"/)
  assert.match(component, /class="contrib-grid contrib-grid-compact"/)
  assert.match(component, /v-for="group in heatmapDayGroups"/)
  assert.match(component, /v-for="label in heatmapBucketTicks"/)
  assert.match(component, /v-for="tick in heatmapHourOffsetTicks"/)
  assert.match(component, /v-for="\(column, columnIndex\) in usageHeatmap"/)
  assert.match(component, /columnIndex % BUCKETS_PER_DAY === 0/)

  assert.match(axisXRule, /display:\s*grid/)
  assert.match(axisYRule, /display:\s*grid/)
  assert.match(axisXGroupsRule, /display:\s*flex/)
  assert.match(axisXGroupsRule, /gap:\s*6px/)
  assert.match(axisXLabelRule, /flex:\s*0 0 49px/)
  assert.match(axisXLabelRule, /font-size:\s*0\.68rem/)
  assert.match(axisYLabelRule, /font-size:\s*0\.68rem/)
  assert.match(bucketAxisRule, /display:\s*grid/)
  assert.match(bucketLabelRule, /flex:\s*0 0 10px/)
  assert.match(bucketLabelRule, /font-size:\s*0\.62rem/)
  assert.match(compactGridRule, /display:\s*flex/)
  assert.match(dayBoundaryRule, /margin-left:\s*3px/)
})
