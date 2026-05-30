<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Dialogs } from '@wailsio/runtime'
import ListItem from '../Setting/ListRow.vue'
import LanguageSwitcher from '../Setting/LanguageSwitcher.vue'
import ThemeSetting from '../Setting/ThemeSetting.vue'
import { fetchAppSettings, saveAppSettings, type AppSettings } from '../../services/appSettings'
import {
  fetchConfigImportStatus,
  fetchConfigImportStatusForFile,
  importFromCcSwitch,
  importFromCustomFile,
  type ConfigImportResult,
  type ConfigImportStatus,
} from '../../services/configImport'
import { showToast } from '../../utils/toast'
import BaseButton from '../common/BaseButton.vue'
import BaseInput from '../common/BaseInput.vue'

const router = useRouter()
const { t } = useI18n()
const DEFAULT_RELAY_PORT = 18101
const heatmapEnabled = ref(true)
const homeTitleVisible = ref(true)
const autoStartEnabled = ref(false)
const relayPort = ref(DEFAULT_RELAY_PORT)
const relayPortInput = ref(String(DEFAULT_RELAY_PORT))
const relayPortError = ref('')
const rawLogCaptureEnabled = ref(false)
const rawLogMaxBytes = ref(262144)
const settingsLoading = ref(true)
const saveBusy = ref(false)
const importStatus = ref<ConfigImportStatus | null>(null)
const customImportStatus = ref<ConfigImportStatus | null>(null)
const importBusy = ref(false)

const goBack = () => {
  router.push('/')
}

const loadAppSettings = async () => {
  settingsLoading.value = true
  try {
    const data = await fetchAppSettings()
    heatmapEnabled.value = data?.show_heatmap ?? true
    homeTitleVisible.value = data?.show_home_title ?? true
    autoStartEnabled.value = data?.auto_start ?? false
    relayPort.value = data?.relay_port ?? DEFAULT_RELAY_PORT
    relayPortInput.value = String(relayPort.value)
    rawLogCaptureEnabled.value = data?.capture_raw_logs ?? false
    rawLogMaxBytes.value = data?.raw_log_max_bytes ?? 262144
    relayPortError.value = ''
  } catch (error) {
    console.error('failed to load app settings', error)
    heatmapEnabled.value = true
    homeTitleVisible.value = true
    autoStartEnabled.value = false
    relayPort.value = DEFAULT_RELAY_PORT
    relayPortInput.value = String(DEFAULT_RELAY_PORT)
    rawLogCaptureEnabled.value = false
    rawLogMaxBytes.value = 262144
    relayPortError.value = ''
  } finally {
    settingsLoading.value = false
  }
}

const persistAppSettings = async (): Promise<boolean> => {
  if (settingsLoading.value || saveBusy.value) return false
  saveBusy.value = true
  try {
    const payload: AppSettings = {
      show_heatmap: heatmapEnabled.value,
      show_home_title: homeTitleVisible.value,
      auto_start: autoStartEnabled.value,
      relay_port: relayPort.value,
      capture_raw_logs: rawLogCaptureEnabled.value,
      raw_log_max_bytes: rawLogMaxBytes.value,
    }
    await saveAppSettings(payload)
    window.dispatchEvent(new CustomEvent('app-settings-updated'))
    return true
  } catch (error) {
    console.error('failed to save app settings', error)
    const message = error instanceof Error ? error.message : String(error)
    showToast(message || t('components.general.relay.saveFailed'), 'error')
    return false
  } finally {
    saveBusy.value = false
  }
}

const relayPortChanged = computed(() => relayPortInput.value.trim() !== String(relayPort.value))
const relayPortSubLabel = computed(() =>
  t('components.general.relay.subLabel', { port: relayPort.value }),
)

const parseRelayPortInput = () => {
  const value = relayPortInput.value.trim()
  if (!/^\d+$/.test(value)) return null
  const port = Number(value)
  if (!Number.isInteger(port) || port < 1024 || port > 65535) return null
  return port
}

const persistRelayPort = async () => {
  relayPortError.value = ''
  const port = parseRelayPortInput()
  if (port === null) {
    relayPortError.value = t('components.general.relay.invalidPort')
    return
  }
  if (port === relayPort.value) return

  const previousPort = relayPort.value
  relayPort.value = port
  const ok = await persistAppSettings()
  if (ok) {
    relayPortInput.value = String(port)
    showToast(t('components.general.relay.restartRequired'))
    return
  }
  relayPort.value = previousPort
}

onMounted(() => {
  void loadAppSettings()
  void loadImportStatus()
})

const loadImportStatus = async () => {
  try {
    importStatus.value = await fetchConfigImportStatus()
  } catch (error) {
    console.error('failed to load cc-switch import status', error)
    importStatus.value = null
  }
}

const activeImportStatus = computed(() => customImportStatus.value ?? importStatus.value)
const hasCustomSelection = computed(() => Boolean(customImportStatus.value))
const shouldShowDefaultMissingHint = computed(() => {
  if (hasCustomSelection.value) return false
  const status = importStatus.value
  if (!status) return false
  return !status.config_exists
})
const pendingProviders = computed(() => activeImportStatus.value?.pending_provider_count ?? 0)
const pendingServers = computed(() => activeImportStatus.value?.pending_mcp_count ?? 0)
const configPath = computed(() => activeImportStatus.value?.config_path ?? '')
const canImportDefault = computed(() => {
  const status = importStatus.value
  if (!status) return false
  return Boolean(status.pending_providers || status.pending_mcp)
})
const canImportCustom = computed(() => {
  const status = customImportStatus.value
  if (!status) return false
  return Boolean(status.pending_providers || status.pending_mcp)
})
const canImportActive = computed(() =>
  hasCustomSelection.value ? canImportCustom.value : canImportDefault.value,
)
const showImportRow = computed(() => Boolean(importStatus.value) || hasCustomSelection.value)
const importPathLabel = computed(() => {
  if (!configPath.value) return ''
  return t('components.general.import.path', { path: configPath.value })
})
const importDetailLabel = computed(() => {
  if (shouldShowDefaultMissingHint.value) {
    return t('components.general.import.missingDefault')
  }
  if (!activeImportStatus.value) {
    return t('components.general.import.noFile')
  }
  const detail = canImportActive.value
    ? t('components.general.import.detail', {
        providers: pendingProviders.value,
        servers: pendingServers.value,
      })
    : t('components.general.import.synced')
  if (!importPathLabel.value) return detail
  return `${importPathLabel.value} · ${detail}`
})
const importButtonText = computed(() => {
  if (importBusy.value) {
    return t('components.general.import.importing')
  }
  if (hasCustomSelection.value) {
    return t('components.general.import.confirm')
  }
  if (shouldShowDefaultMissingHint.value || canImportDefault.value) {
    return t('components.general.import.cta')
  }
  return t('components.general.import.syncedButton')
})
const primaryButtonDisabled = computed(() => importBusy.value || !canImportActive.value)
const secondaryButtonLabel = computed(() =>
  hasCustomSelection.value
    ? t('components.general.import.clear')
    : t('components.general.import.upload'),
)
const secondaryButtonVariant = computed(() => 'outline' as const)

const processImportResult = async (result?: ConfigImportResult | null) => {
  if (!result) return
  if (hasCustomSelection.value && result.status?.config_path === customImportStatus.value?.config_path) {
    customImportStatus.value = result.status
  } else {
    importStatus.value = result.status
  }
  const importedProviders = result.imported_providers ?? 0
  const importedServers = result.imported_mcp ?? 0
  if (importedProviders > 0 || importedServers > 0) {
    showToast(
      t('components.main.importConfig.success', {
        providers: importedProviders,
        servers: importedServers,
      })
    )
  } else if (result.status?.config_exists) {
    showToast(t('components.main.importConfig.empty'))
  }
  await loadImportStatus()
}

const handleImportClick = async () => {
  if (importBusy.value || !importStatus.value || !canImportDefault.value) return
  importBusy.value = true
  try {
    const result = await importFromCcSwitch()
    await processImportResult(result)
  } catch (error) {
    console.error('failed to import cc-switch config', error)
    showToast(t('components.main.importConfig.error'), 'error')
  } finally {
    importBusy.value = false
  }
}

const handleConfirmCustomImport = async () => {
  const path = customImportStatus.value?.config_path
  if (!path || importBusy.value || !canImportCustom.value) return
  importBusy.value = true
  try {
    const result = await importFromCustomFile(path)
    await processImportResult(result)
  } catch (error) {
    console.error('failed to import custom cc-switch config', error)
    showToast(t('components.main.importConfig.error'), 'error')
  } finally {
    importBusy.value = false
  }
}

const handlePrimaryImport = async () => {
  if (hasCustomSelection.value) {
    await handleConfirmCustomImport()
  } else {
    await handleImportClick()
  }
}

const handleUploadClick = async () => {
  if (importBusy.value) return
  let selectedPath = ''
  try {
    const selection = await Dialogs.OpenFile({
      Title: t('components.general.import.uploadTitle'),
      CanChooseFiles: true,
      CanChooseDirectories: false,
      AllowsOtherFiletypes: false,
      Filters: [
        {
          DisplayName: 'JSON (*.json)',
          Pattern: '*.json',
        },
      ],
      AllowsMultipleSelection: false,
    })
    selectedPath = Array.isArray(selection) ? selection[0] : selection
    if (!selectedPath) return
    const status = await fetchConfigImportStatusForFile(selectedPath)
    customImportStatus.value = status
  } catch (error) {
    console.error('failed to load custom cc-switch config status', error)
    showToast(t('components.general.import.loadError'), 'error')
  }
}

const clearCustomSelection = () => {
  customImportStatus.value = null
}

const handleSecondaryImportAction = async () => {
  if (hasCustomSelection.value) {
    clearCustomSelection()
  } else {
    await handleUploadClick()
  }
}
</script>

<template>
  <div class="main-shell general-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ $t('components.general.title.application') }}</p>
      <button class="ghost-icon" :aria-label="$t('components.general.buttons.back')" @click="goBack">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M15 18l-6-6 6-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>

    <div class="general-page">
      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.application') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.heatmap')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="heatmapEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.homeTitle')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="homeTitleVisible"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.autoStart')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="autoStartEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem
            :label="$t('components.general.label.relayPort')"
            :sub-label="relayPortSubLabel"
          >
            <div class="relay-port-control">
              <BaseInput
                v-model="relayPortInput"
                class="relay-port-input"
                type="text"
                inputmode="numeric"
                pattern="[0-9]*"
                :disabled="settingsLoading || saveBusy"
                @keydown.enter.prevent="persistRelayPort"
              />
              <BaseButton
                size="sm"
                variant="outline"
                type="button"
                :disabled="settingsLoading || saveBusy || !relayPortChanged"
                @click="persistRelayPort"
              >
                {{ $t('components.general.relay.save') }}
              </BaseButton>
              <span v-if="relayPortError" class="relay-port-error">{{ relayPortError }}</span>
            </div>
          </ListItem>
          <ListItem
            :label="$t('components.general.label.rawLogs')"
            :sub-label="$t('components.general.rawLogs.subLabel')"
          >
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="rawLogCaptureEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem
            v-if="showImportRow"
            :label="$t('components.general.import.label')"
            :sub-label="importDetailLabel"
          >
            <div class="import-actions">
              <BaseButton
                size="sm"
                variant="outline"
                type="button"
                :disabled="primaryButtonDisabled"
                @click="handlePrimaryImport"
              >
                {{ importButtonText }}
              </BaseButton>
              <BaseButton
                size="sm"
                :variant="secondaryButtonVariant"
                type="button"
                :disabled="importBusy"
                @click="handleSecondaryImportAction"
              >
                {{ secondaryButtonLabel }}
              </BaseButton>
              <BaseButton
                v-if="hasCustomSelection"
                size="sm"
                variant="outline"
                type="button"
                :disabled="importBusy"
                @click="handleUploadClick"
              >
                {{ $t('components.general.import.reupload') }}
              </BaseButton>
            </div>
          </ListItem>

        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.exterior') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.language')">
            <LanguageSwitcher />
          </ListItem>
          <ListItem :label="$t('components.general.label.theme')">
            <ThemeSetting />
          </ListItem>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.import-actions {
  display: flex;
  gap: 0.35rem;
  justify-content: flex-end;
  flex-wrap: wrap;
}

.import-actions .btn {
  min-width: 56px;
  padding: 0.3rem 0.75rem;
  font-size: 0.7rem;
}

.import-actions .btn-outline,
.import-actions .btn-ghost {
  padding-inline: 0.75rem;
}

.relay-port-control {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.relay-port-input {
  width: 96px;
  text-align: center;
}

.relay-port-error {
  flex-basis: 100%;
  text-align: right;
  color: #d64545;
  font-size: 0.72rem;
}
</style>
