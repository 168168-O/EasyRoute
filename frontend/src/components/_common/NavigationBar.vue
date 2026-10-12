<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import rawRoutes from '@/router/routes'
import { useAppSettingsStore } from '@/stores'

const { t } = useI18n()
const appSettings = useAppSettingsStore()

const SIMPLE_NAV = ['Overview', 'AppRouting', 'Subscriptions', 'Settings', 'Translate']

const routes = computed(() => {
  const visible = rawRoutes.filter(
    (r) =>
      r.meta?.hidden === false ||
      (!r.meta?.hidden && appSettings.app.pages.includes(r.name! as string)),
  )
  if (appSettings.app.simpleMode === false) return visible
  return visible.filter((route) => SIMPLE_NAV.includes(String(route.name)))
})

const icons: Record<string, string> = {
  Overview:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7.5" height="7.5" rx="2"/><rect x="13.5" y="3" width="7.5" height="7.5" rx="2"/><rect x="3" y="13.5" width="7.5" height="7.5" rx="2"/><rect x="13.5" y="13.5" width="7.5" height="7.5" rx="2"/></svg>',
  AppRouting:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12h6l4-6h6M14 18h6M10 12l4 6"/><path d="M17 3l3 3-3 3M17 15l3 3-3 3"/></svg>',
  Translate:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 5h8M8 3v2c0 4-2.5 7-6 8"/><path d="M5 9c.8 2.2 2.4 4 5 5.2"/><path d="M13 20l4-9 4 9M14.2 17h5.6"/></svg>',
  Profiles:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2.5 8 12 13l9.5-5z"/><path d="M2.5 12.5 12 17.5l9.5-5M2.5 16.5 12 21.5l9.5-5"/></svg>',
  Subscriptions:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3.5" y="3.5" width="17" height="17" rx="3"/><path d="M8 12l3 3 5-6"/></svg>',
  Rulesets:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6 4h12v16H6z"/><path d="M9 8h6M9 12h6M9 16h4"/></svg>',
  Plugins:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2.8 20.5 7.5v9L12 21.2 3.5 16.5v-9z"/><path d="M3.5 7.5 12 12l8.5-4.5M12 12v9.2"/></svg>',
  ScheduledTasks:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="13" height="17" rx="2.5"/><path d="M8 3v3M13 3v3M7.5 10h6M7.5 14h3"/><circle cx="17.5" cy="17" r="4"/><path d="M17.5 15.3V17l1.2 1"/></svg>',
  Settings:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3.2"/><path d="M12 2.5l1.6 2.3 2.7-.6.7 2.7 2.6 1-.8 2.6 1.8 2.1-2.2 1.6.2 2.8-2.8.2-1.3 2.4L12 20.4l-2.5 1.2-1.3-2.4-2.8-.2.2-2.8L3.4 14.6l1.8-2.1-.8-2.6 2.6-1 .7-2.7 2.7.6z"/></svg>',
}
</script>

<template>
  <div class="main-nav flex items-center justify-center">
    <div
      v-for="r in routes"
      :key="r.path"
      :class="['main-nav__item', `main-nav__item--${String(r.name).toLowerCase()}`]"
    >
      <RouterLink v-slot="{ navigate, isActive }" :to="r.path" custom>
        <Button
          :class="{ 'is-active': isActive }"
          class="main-nav__button"
          :type="isActive ? 'link' : 'text'"
          @click="navigate"
        >
          <span class="nav-ico" v-html="icons[String(r.name)] || ''" />
          {{ (r.meta && t(r.meta.name)) || r.name }}
        </Button>
      </RouterLink>
    </div>
  </div>
</template>
