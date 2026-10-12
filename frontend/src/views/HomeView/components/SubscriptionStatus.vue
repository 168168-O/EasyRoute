<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useSubscribesStore } from '@/stores'
import { formatBytes } from '@/utils'
import { subscriptionHealth, type SubscriptionHealth } from '@/utils/subscriptionStatus'

const { t } = useI18n()
const store = useSubscribesStore()

const rows = computed(() =>
  (store.subscribes || [])
    .map((sub) => subscriptionHealth(sub))
    .filter((row): row is SubscriptionHealth => row != null),
)

const known = computed(() => rows.value.filter((row) => row.total > 0 || row.daysLeft != null))
const warning = computed(() => known.value.filter((row) => row.warn))

const remainText = (row: SubscriptionHealth) =>
  row.total > 0 ? `${formatBytes(row.remain)} / ${formatBytes(row.total)}` : t('home.subscription.noTraffic')

const expireText = (row: SubscriptionHealth) => {
  if (row.daysLeft == null) return t('home.subscription.noExpire')
  if (row.daysLeft < 0) return t('home.subscription.expired')
  return t('home.subscription.daysLeft', [row.daysLeft])
}

const warnText = (row: SubscriptionHealth) => {
  if (row.reason === 'both') return t('home.subscription.warnBoth')
  if (row.reason === 'expire') return t('home.subscription.warnExpire')
  return t('home.subscription.warnTraffic')
}
</script>

<template>
  <div v-if="rows.length" class="sub-status">
    <div v-if="warning.length" class="sub-warn">
      <div v-for="(row, index) in warning" :key="`${row.name}-${index}`">{{ row.name }} · {{ warnText(row) }}</div>
    </div>
    <div v-for="(row, index) in known" :key="`${row.name}-${index}`" class="sub-row">
      <span class="sub-name">{{ row.name }}</span>
      <span class="sub-value">
        {{ t('home.subscription.remain') }} {{ remainText(row) }} · {{ t('home.subscription.expire') }}
        {{ expireText(row) }}
      </span>
    </div>
    <p v-if="!known.length" class="sub-idle">{{ t('home.subscription.unknown') }}</p>
  </div>
</template>
